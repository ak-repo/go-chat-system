package websocket

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/ak-repo/go-chat-system/service"
)

type Hub struct {
	clients        map[string]map[*Client]bool
	rooms          map[string]*Room
	register       chan *Client
	unregister     chan *Client
	incoming       chan *WSMessage
	messageService service.MessageService
}

func NewHub(msgService service.MessageService) *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		incoming:       make(chan *WSMessage),
		messageService: msgService,
	}
}

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

	if h.messageService != nil && msg.ReceiverType == ReceiverUser {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var payload struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(msg.Data, &payload); err != nil {
				log.Printf("failed to parse message data: %v", err)
				return
			}

			_, err := h.messageService.CreateMessage(ctx, msg.SenderID, msg.ReceiverID, payload.Text, false)
			if err != nil {
				log.Printf("failed to persist message: %v", err)
			}
		}()
	}

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
			close(c.send)
			delete(conns, c)
		}
	}
}
