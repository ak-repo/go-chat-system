package websocket

import (
	"log"

	"github.com/gorilla/websocket"
)

type Client struct {
	userID string
	conn   *websocket.Conn
	send   chan *WSMessage
	hub    *Hub
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan *WSMessage, 256),
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
		var msg WSMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			log.Println("msg read err: ", err)
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
