package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/ak-repo/go-chat-system/internal/service"
)

type Hub struct {
	clients        map[string]map[*Client]bool
	rooms          map[string]*Room
	register       chan *Client
	unregister     chan *Client
	incoming       chan *WSMessage
	messageService service.MessageService
	// Graceful shutdown support
	quit chan struct{}
}

func NewHub(msgService service.MessageService) *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]bool),
		rooms:          make(map[string]*Room),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		incoming:       make(chan *WSMessage),
		messageService: msgService,
		quit:           make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case <-h.quit:
			// Graceful shutdown: close all client connections
			for userID, conns := range h.clients {
				for c := range conns {
					close(c.send)
				}
				delete(h.clients, userID)
			}
			return

		case c := <-h.register:
			conns, ok := h.clients[c.userID]
			if !ok {
				conns = make(map[*Client]bool)
				h.clients[c.userID] = conns
			}
			conns[c] = true
			h.broadcastPresence(c.userID, "user_online")

		case c := <-h.unregister:
			userID := c.userID
			if conns, ok := h.clients[userID]; ok {
				delete(conns, c)
				if len(conns) == 0 {
					delete(h.clients, userID)
					h.broadcastPresence(userID, "user_offline")
				}
			}
			close(c.send)

		case msg := <-h.incoming:
			h.routeMessage(msg)
		}
	}
}

// Stop gracefully shuts down the hub
func (h *Hub) Stop() {
	close(h.quit)
}

func (h *Hub) Register(client *Client) {
	log.Println("client registered: ", client.userID)
	h.register <- client
}

func (h *Hub) broadcastPresence(userID, event string) {
	msg := WSMessage{
		Event:    event,
		SenderID: userID,
	}
	h.broadcastToAll(msg)
}

func (h *Hub) broadcastToAll(msg WSMessage) {
	for userID, conns := range h.clients {
		for c := range conns {
			select {
			case c.send <- &msg:
			default:
				close(c.send)
				delete(conns, c)
			}
		}
		_ = userID
	}
}

func (h *Hub) routeMessage(msg *WSMessage) {
	switch msg.Event {
	case "user_online":
		h.broadcastPresence(msg.SenderID, "user_online")
		return
	case "user_offline":
		h.broadcastPresence(msg.SenderID, "user_offline")
		return
	}

	if msg.Event == "message" && msg.ReceiverType == ReceiverUser {
		h.handleUserMessage(msg)
		return
	}

	switch msg.ReceiverType {
	case ReceiverUser:
		h.sendToUser(msg)
	case ReceiverGroup:
		h.sendToGroup(msg)
	}
}

func (h *Hub) handleUserMessage(msg *WSMessage) {
	if h.messageService == nil {
		h.sendErrorToUser(msg.SenderID, "message service unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	text, err := extractMessageText(msg.Data)
	if err != nil {
		log.Printf("failed to parse message data: %v", err)
		h.sendErrorToUser(msg.SenderID, "message text missing")
		return
	}

	persisted, err := h.messageService.CreateMessage(ctx, msg.SenderID, msg.ReceiverID, text, false)
	if err != nil {
		log.Printf("failed to persist message: %v", err)
		h.sendErrorToUser(msg.SenderID, "message could not be sent")
		return
	}

	data, err := json.Marshal(map[string]string{
		"message_id": persisted.ID,
		"content":    persisted.Body,
		"timestamp":  persisted.CreatedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		log.Printf("failed to marshal message data: %v", err)
		h.sendErrorToUser(msg.SenderID, "message could not be sent")
		return
	}

	outbound := &WSMessage{
		Event:        "message",
		SenderID:     persisted.SenderID,
		ReceiverID:   persisted.ReceiverID,
		ReceiverType: ReceiverUser,
		Data:         data,
	}
	h.sendToUser(outbound)

	ackData, _ := json.Marshal(map[string]string{
		"message_id": persisted.ID,
		"status":     "sent",
	})
	h.sendToUser(&WSMessage{Event: "ack", SenderID: "system", ReceiverID: persisted.SenderID, ReceiverType: ReceiverUser, Data: ackData})
}

func (h *Hub) sendErrorToUser(userID, message string) {
	data, _ := json.Marshal(map[string]string{"message": message})
	h.sendToUser(&WSMessage{Event: "error", SenderID: "system", ReceiverID: userID, ReceiverType: ReceiverUser, Data: data})
}

func extractMessageText(data json.RawMessage) (string, error) {
	var payload struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if payload.Text != "" {
		return payload.Text, nil
	}
	if payload.Content != "" {
		return payload.Content, nil
	}
	return "", errors.New("message text missing")
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
			close(c.send)
			delete(conns, c)
		}
	}
}
