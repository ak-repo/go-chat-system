package websocket

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// simpleRateLimiter allows at most N messages per second per client.
type simpleRateLimiter struct {
	mu       sync.Mutex
	count    int
	windowAt time.Time
	limit    int
}

func (s *simpleRateLimiter) allow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if now.Sub(s.windowAt) >= time.Second {
		s.windowAt = now
		s.count = 0
	}
	s.count++
	return s.count <= s.limit
}

type Client struct {
	userID       string
	conn         *websocket.Conn
	send         chan *WSMessage
	hub          *Hub
	readDeadline time.Duration
	rateLimit    *simpleRateLimiter
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string, readDeadlineSec, messagesPerSec int) *Client {
	c := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan *WSMessage, 256),
		userID: userID,
	}
	if readDeadlineSec > 0 {
		c.readDeadline = time.Duration(readDeadlineSec) * time.Second
	}
	if messagesPerSec > 0 {
		c.rateLimit = &simpleRateLimiter{limit: messagesPerSec}
	}
	return c
}

// Convert socket input → domain message → hub channel (client → hub)
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		if c.readDeadline > 0 {
			if err := c.conn.SetReadDeadline(time.Now().Add(c.readDeadline)); err != nil {
				return
			}
		}
		var msg WSMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			log.Println("msg read err: ", err)
			return
		}
		if c.rateLimit != nil && !c.rateLimit.allow() {
			log.Println("ws rate limit exceeded, closing: ", c.userID)
			return
		}
		log.Println("read pump: ", msg.ReceiverID)

		msg.SenderID = c.userID
		c.hub.incoming <- &msg
	}
}

// This is the only place that writes to WebSocket.
func (c *Client) WritePump() {
	defer c.conn.Close()

	for msg := range c.send {
		log.Println("WRITE →", msg.SenderID, "->", msg.ReceiverID)
		if err := c.conn.WriteJSON(msg); err != nil {
			log.Println("msg write err: ", err)
			return
		}
	}
}
