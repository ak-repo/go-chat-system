package websocket

import "github.com/ak-repo/go-chat-system/internal/domain"

type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *domain.Message
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *domain.Message),
	}
}

// This loop is the entire chat system.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c.userID] = c

		case c := <-h.unregister:
			delete(h.clients, c.userID)
			close(c.send)

		case msg := <-h.broadcast:
			if target, ok := h.clients[msg.ReceiverID]; ok {
				target.send <- msg
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}
