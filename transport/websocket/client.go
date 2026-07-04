package websocket

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MaxMessageSize  = 1024 * 10 // 10KB max message size
	ReadDeadline    = 60 * time.Second
	WriteDeadline   = 10 * time.Second
	PingPeriod      = 30 * time.Second
	RateLimitCount  = 10 // max messages per window
	RateLimitWindow = 1 * time.Second
)

type Client struct {
	userID      string
	conn        *websocket.Conn
	send        chan *WSMessage
	hub         *Hub
	rateLimiter *RateLimiter
}

type RateLimiter struct {
	mu        sync.Mutex
	count     int
	resetTime time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		count:     0,
		resetTime: time.Now().Add(RateLimitWindow),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.After(r.resetTime) {
		r.count = 0
		r.resetTime = now.Add(RateLimitWindow)
	}

	if r.count >= RateLimitCount {
		return false
	}
	r.count++
	return true
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan *WSMessage, 256),
		userID:      userID,
		rateLimiter: NewRateLimiter(),
	}
}

// Convert socket input → domain message → hub channel (client → hub)
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(ReadDeadline))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(ReadDeadline))
		return nil
	})

	for {
		var msg WSMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS read error: %v", err)
			}
			return
		}

		if !c.rateLimiter.Allow() {
			log.Printf("Rate limit exceeded for user: %s", c.userID)
			c.conn.WriteJSON(WSMessage{
				Event:    "error",
				Data:     []byte(`{"message":"rate limit exceeded"}`),
				SenderID: "system",
			})
			continue
		}

		msg.SenderID = c.userID
		c.hub.incoming <- &msg
	}
}

// This is the only place that writes to WebSocket.
func (c *Client) WritePump() {
	defer c.conn.Close()

	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(WriteDeadline))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(msg); err != nil {
				log.Printf("msg write err: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(WriteDeadline))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
