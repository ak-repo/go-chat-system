package websocket

import "log"

type Hub struct {
	clients    map[string]map[*Client]bool
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	incoming   chan *WSMessage
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan *WSMessage),
	}
}

// This loop is the entire chat system.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			conns, ok := h.clients[c.userID]
			if !ok {
				conns = make(map[*Client]bool)
				h.clients[c.userID] = conns
			}
			conns[c] = true

		case c := <-h.unregister:
			if conns, ok := h.clients[c.userID]; ok {
				delete(conns, c)
				if len(conns) == 0 {
					delete(h.clients, c.userID)
				}
			}
			close(c.send)

		case msg := <-h.incoming:
			h.routeMessage(msg)
		}
	}
}

func (h *Hub) Register(client *Client) {
	log.Println("client registered: ", client.userID)
	h.register <- client
}

func (h *Hub) routeMessage(msg *WSMessage) {
	switch msg.ReceiverType {

	case ReceiverUser:
		h.sendToUser(msg)

	case ReceiverGroup:
		h.sendToGroup(msg)
	}
}

func (h *Hub) sendToUser(msg *WSMessage) {
	conns, ok := h.clients[msg.ReceiverID]
	if !ok {
		return
	}

	for c := range conns {
		select {
		case c.send <- msg:
		default:
			// Drop slow client
			close(c.send)
			delete(conns, c)
		}
	}
}
