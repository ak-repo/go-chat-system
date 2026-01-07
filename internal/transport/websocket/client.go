package websocket

import (
	"github.com/ak-repo/go-chat-system/internal/domain"
	"github.com/gorilla/websocket"
)

type Client struct {
	userID string
	conn   *websocket.Conn
	send   chan *domain.Message
	hub    *Hub
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan *domain.Message, 256),
		userID: userID,
	}
}

// Convert socket input → domain message → hub channel (client → hub)
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		var msg domain.Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}

		msg.SenderID = c.userID
		c.hub.broadcast <- &msg
	}
}

// This is the only place that writes to WebSocket.
func (c *Client) WritePump() {
	for msg := range c.send {
		if err := c.conn.WriteJSON(msg); err != nil {
			return
		}
	}
}
