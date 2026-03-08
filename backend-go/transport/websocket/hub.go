package websocket

import (
	"encoding/json"
	"log"
)

// PersistDMFunc persists a DM message and returns the message to broadcast (with id, chat_id, etc.). If nil, hub does not persist.
type PersistDMFunc func(senderID, receiverID, content string) (*WSMessage, error)

type Hub struct {
	clients    map[string]map[*Client]bool
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	incoming   chan *WSMessage
	PersistDM  PersistDMFunc
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
		if h.PersistDM != nil {
			content := extractContent(msg.Data)
			if content != "" {
				persisted, err := h.PersistDM(msg.SenderID, msg.ReceiverID, content)
				if err == nil && persisted != nil {
					h.sendToUserID(persisted, msg.SenderID)
					h.sendToUserID(persisted, msg.ReceiverID)
					return
				}
			}
		}
		h.sendToUser(msg)
	case ReceiverGroup:
		h.sendToGroup(msg)
	}
}

func (h *Hub) sendToUser(msg *WSMessage) {
	h.sendToUserID(msg, msg.ReceiverID)
}

func extractContent(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var v struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	if v.Content != "" {
		return v.Content
	}
	return v.Text
}

func (h *Hub) sendToUserID(msg *WSMessage, userID string) {
	conns, ok := h.clients[userID]
	if !ok {
		return
	}
	for c := range conns {
		select {
		case c.send <- msg:
		default:
			close(c.send)
			delete(conns, c)
		}
	}
}
