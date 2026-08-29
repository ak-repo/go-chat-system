package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/service"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
)

const persistenceTimeout = 5 * time.Second

type Hub struct {
	clients        map[string]map[*Client]bool
	rooms          map[string]*Room
	register       chan *Client
	unregister     chan *Client
	incoming       chan *WSMessage
	results        chan *deliveryResult
	messageService service.MessageService
	quit           chan struct{}
	stopOnce       chan struct{}
	activeSession  func(context.Context, string, string) bool
}

type deliveryResult struct {
	actor   string
	message *WSMessage
	err     error
}

func (h *Hub) sendResult(result *deliveryResult) {
	select {
	case h.results <- result:
	case <-h.quit:
	}
}

func NewHub(msgService service.MessageService) *Hub {
	return &Hub{clients: make(map[string]map[*Client]bool), rooms: make(map[string]*Room), register: make(chan *Client), unregister: make(chan *Client), incoming: make(chan *WSMessage), results: make(chan *deliveryResult, 64), messageService: msgService, quit: make(chan struct{}), stopOnce: make(chan struct{}, 1)}
}

func (h *Hub) Run() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.quit:
			for uid, conns := range h.clients {
				for c := range conns {
					c.closeSend()
					delete(conns, c)
				}
				delete(h.clients, uid)
			}
			return
		case c := <-h.register:
			wasOffline := len(h.clients[c.userID]) == 0
			if h.clients[c.userID] == nil {
				h.clients[c.userID] = make(map[*Client]bool)
			}
			h.clients[c.userID][c] = true
			if wasOffline {
				h.broadcastPresence(c.userID, "user_online")
			}
		case c := <-h.unregister:
			wasOnline := len(h.clients[c.userID]) > 0
			h.remove(c)
			if wasOnline && len(h.clients[c.userID]) == 0 {
				h.broadcastPresence(c.userID, "user_offline")
			}
		case msg := <-h.incoming:
			h.dispatch(msg) // never performs database work on this loop
		case result := <-h.results:
			if result.err != nil {
				h.sendDomainError(result.actor, result.err)
				continue
			}
			h.sendToUser(result.message)
			var p map[string]any
			_ = json.Unmarshal(result.message.Data, &p)
			status := "sent"
			if result.message.Event == EventDelivered {
				status = "delivered"
			}
			if result.message.Event == EventRead {
				status = "read"
			}
			serverID := p["server_id"]
			if serverID == nil || serverID == "" {
				serverID = p["message_id"]
			}
			ack, _ := json.Marshal(map[string]any{"server_id": serverID, "client_message_id": p["client_message_id"], "status": status, "event": result.message.Event})
			h.sendToUser(&WSMessage{Event: EventAck, ReceiverID: result.actor, ReceiverType: ReceiverUser, Data: ack})
			if result.actor != result.message.ReceiverID {
				copy := *result.message
				copy.ReceiverID = result.actor
				h.sendToUser(&copy)
			}
		case <-ticker.C:
			if h.activeSession != nil {
				for _, conns := range h.clients {
					for c := range conns {
						if !h.activeSession(context.Background(), c.userID, c.sessionID) {
							c.conn.Close()
						}
					}
				}
			}
		}
	}
}
func (h *Hub) SetSessionChecker(check func(context.Context, string, string) bool) {
	h.activeSession = check
}

func (h *Hub) Stop() {
	select {
	case h.stopOnce <- struct{}{}:
		close(h.quit)
	default:
	}
}
func (h *Hub) Register(c *Client) {
	select {
	case h.register <- c:
	case <-h.quit:
		c.closeSend()
	}
}
func (h *Hub) remove(c *Client) {
	if conns := h.clients[c.userID]; conns != nil {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.clients, c.userID)
		}
	}
	c.closeSend()
}

func (h *Hub) dispatch(msg *WSMessage) {
	if msg == nil {
		return
	}
	actor := msg.actorID
	if actor == "" {
		actor = msg.SenderID
	} // compatibility for in-package callers
	msg.SenderID = actor
	if err := msg.Validate(); err != nil {
		h.sendErrorToUser(actor, err.Error(), "invalid_event")
		return
	}
	if msg.Event == EventTyping {
		var p struct {
			ConversationID string `json:"conversation_id"`
			State          bool   `json:"state"`
		}
		if json.Unmarshal(msg.Data, &p) != nil {
			h.sendErrorToUser(actor, "invalid event data", "invalid_event")
			return
		}
		if s, ok := h.messageService.(interface {
			AuthorizeRealtime(context.Context, string, service.RealtimeEvent) error
		}); ok {
			ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
			err := s.AuthorizeRealtime(ctx, actor, service.RealtimeEvent{Event: msg.Event, ConversationID: p.ConversationID, ReceiverID: msg.ReceiverID})
			cancel()
			if err != nil {
				h.sendErrorToUser(actor, "not authorized", "forbidden")
				return
			}
		} else {
			h.sendErrorToUser(actor, "event is not supported", "unsupported_event")
			return
		}
		h.sendToUser(msg)
		return
	} // deliberately ephemeral
	if msg.Event != EventMessage {
		h.dispatchMutation(msg, actor)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
		defer cancel()
		text, clientID, err := extractMessage(msg.Data)
		if err != nil {
			h.sendResult(&deliveryResult{actor: actor, err: err})
			return
		}
		var persisted *model.Message
		if s, ok := h.messageService.(interface {
			CreateMessageWithClientID(context.Context, string, string, string, string, bool) (*model.Message, error)
		}); ok {
			persisted, err = s.CreateMessageWithClientID(ctx, actor, msg.ReceiverID, text, clientID, false)
		} else {
			persisted, err = h.messageService.CreateMessage(ctx, actor, msg.ReceiverID, text, false)
		}
		if err != nil {
			h.sendResult(&deliveryResult{actor: actor, err: err})
			return
		}
		data, _ := json.Marshal(map[string]any{"message_id": persisted.ID, "client_message_id": clientID, "content": persisted.Body, "timestamp": persisted.CreatedAt.Format(time.RFC3339Nano)})
		h.sendResult(&deliveryResult{actor: actor, message: &WSMessage{Event: EventMessage, SenderID: actor, ReceiverID: msg.ReceiverID, ReceiverType: ReceiverUser, Data: data}})
	}()
}

func (h *Hub) dispatchMutationSync(msg *WSMessage, actor string) {
	if s, ok := h.messageService.(interface {
		HandleRealtimeResult(context.Context, string, service.RealtimeEvent) (*service.RealtimeResult, error)
	}); ok {
		h.persistMutation(msg, actor, s.HandleRealtimeResult)
		return
	}
	s, ok := h.messageService.(interface {
		HandleRealtime(context.Context, string, service.RealtimeEvent) error
	})
	if !ok {
		h.sendErrorToUser(actor, "event is not supported", "unsupported_event")
		return
	}
	var p struct {
		ConversationID string `json:"conversation_id"`
		MessageID      string `json:"message_id"`
		Content        string `json:"content"`
	}
	if json.Unmarshal(msg.Data, &p) != nil {
		h.sendErrorToUser(actor, "invalid event data", "invalid_event")
		return
	}
	e := service.RealtimeEvent{Event: msg.Event, ConversationID: p.ConversationID, MessageID: p.MessageID, Content: p.Content, ReceiverID: msg.ReceiverID}
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()
	if err := s.HandleRealtime(ctx, actor, e); err != nil {
		h.sendDomainError(actor, err)
		return
	}
	h.sendToUser(msg)
	if actor != msg.ReceiverID {
		copy := *msg
		copy.ReceiverID = actor
		h.sendToUser(&copy)
	}
}

// routeMessage remains synchronous for callers that use the transport router
// directly; Run uses dispatch so persistence cannot stall unrelated sockets.
func (h *Hub) routeMessage(msg *WSMessage) { h.dispatchSync(msg) }
func (h *Hub) dispatchSync(msg *WSMessage) {
	actor := msg.actorID
	if actor == "" {
		actor = msg.SenderID
	}
	msg.SenderID = actor
	if err := msg.Validate(); err != nil {
		h.sendErrorToUser(actor, err.Error(), "invalid_event")
		return
	}
	if msg.Event != EventMessage {
		h.dispatchMutationSync(msg, actor)
		return
	}
	text, cid, err := extractMessage(msg.Data)
	if err != nil {
		h.sendErrorToUser(actor, err.Error(), "invalid_event")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()
	var persisted *model.Message
	if s, ok := h.messageService.(interface {
		CreateMessageWithClientID(context.Context, string, string, string, string, bool) (*model.Message, error)
	}); ok {
		persisted, err = s.CreateMessageWithClientID(ctx, actor, msg.ReceiverID, text, cid, false)
	} else {
		persisted, err = h.messageService.CreateMessage(ctx, actor, msg.ReceiverID, text, false)
	}
	if err != nil {
		h.sendDomainError(actor, err)
		return
	}
	data, _ := json.Marshal(map[string]any{"message_id": persisted.ID, "client_message_id": cid, "content": persisted.Body, "timestamp": persisted.CreatedAt.Format(time.RFC3339Nano)})
	out := &WSMessage{Event: EventMessage, SenderID: actor, ReceiverID: msg.ReceiverID, ReceiverType: ReceiverUser, Data: data}
	h.sendToUser(out)
	ack, _ := json.Marshal(map[string]any{"server_id": persisted.ID, "client_message_id": cid, "status": "sent"})
	h.sendToUser(&WSMessage{Event: EventAck, ReceiverID: actor, ReceiverType: ReceiverUser, Data: ack})
	if actor != msg.ReceiverID {
		copy := *out
		copy.ReceiverID = actor
		h.sendToUser(&copy)
	}
}

func (h *Hub) dispatchMutation(msg *WSMessage, actor string) {
	// Mutation/status methods are intentionally optional for old service
	// implementations; production MessageServiceImpl implements them.
	if s, ok := h.messageService.(interface {
		HandleRealtimeResult(context.Context, string, service.RealtimeEvent) (*service.RealtimeResult, error)
	}); ok {
		p, err := mutationPayload(msg)
		if err != nil {
			h.sendErrorToUser(actor, "invalid event data", "invalid_event")
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
			defer cancel()
			result, err := s.HandleRealtimeResult(ctx, actor, service.RealtimeEvent{Event: msg.Event, ConversationID: p.ConversationID, MessageID: p.MessageID, Content: p.Content, ReceiverID: msg.ReceiverID})
			if err != nil {
				h.sendResult(&deliveryResult{actor: actor, err: err})
				return
			}
			data, _ := json.Marshal(map[string]string{"message_id": result.MessageID, "conversation_id": result.ConversationID, "content": result.Content, "server_id": result.ServerID})
			h.sendResult(&deliveryResult{actor: actor, message: &WSMessage{Event: result.Event, SenderID: actor, ReceiverID: result.ReceiverID, ReceiverType: ReceiverUser, Data: data}})
		}()
		return
	}
	// Legacy persistence-only implementations cannot provide the canonical
	// target/message identity required for safe realtime mutation delivery.
	h.sendErrorToUser(actor, "event is not supported", "unsupported_event")
	return
}

type mutationData struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	Content        string `json:"content"`
}

func mutationPayload(msg *WSMessage) (mutationData, error) {
	var p mutationData
	if err := json.Unmarshal(msg.Data, &p); err != nil {
		return p, err
	}
	return p, nil
}

func (h *Hub) persistMutation(msg *WSMessage, actor string, persist func(context.Context, string, service.RealtimeEvent) (*service.RealtimeResult, error)) {
	p, err := mutationPayload(msg)
	if err != nil {
		h.sendErrorToUser(actor, "invalid event data", "invalid_event")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()
	result, err := persist(ctx, actor, service.RealtimeEvent{Event: msg.Event, ConversationID: p.ConversationID, MessageID: p.MessageID, Content: p.Content, ReceiverID: msg.ReceiverID})
	if err != nil {
		h.sendDomainError(actor, err)
		return
	}
	data, _ := json.Marshal(map[string]string{"message_id": result.MessageID, "conversation_id": result.ConversationID, "content": result.Content, "server_id": result.ServerID})
	out := &WSMessage{Event: result.Event, SenderID: actor, ReceiverID: result.ReceiverID, ReceiverType: ReceiverUser, Data: data}
	h.sendToUser(out)
	if actor != result.ReceiverID {
		copy := *out
		copy.ReceiverID = actor
		h.sendToUser(&copy)
	}
}

func (h *Hub) sendErrorToUser(userID, message, code string) {
	data, _ := json.Marshal(map[string]string{"code": code, "message": message})
	h.sendToUser(&WSMessage{Event: EventError, ReceiverID: userID, ReceiverType: ReceiverUser, Data: data})
}
func (h *Hub) sendDomainError(userID string, err error) {
	code, message := "internal_error", "event could not be processed"
	if errors.Is(err, errs.ErrValidation) || errors.Is(err, errs.ErrBadRequest) {
		code, message = "invalid_event", "invalid event"
	}
	if errors.Is(err, errs.ErrForbidden) || errors.Is(err, errs.ErrBlockedRelationship) || errors.Is(err, errs.ErrNotMember) {
		code, message = "forbidden", "not authorized"
	}
	if errors.Is(err, errs.ErrNotFound) {
		code, message = "not_found", "message not found"
	}
	data, _ := json.Marshal(map[string]any{"code": code, "message": message, "retryable": code == "internal_error"})
	h.sendToUser(&WSMessage{Event: EventError, ReceiverID: userID, ReceiverType: ReceiverUser, Data: data})
}
func (h *Hub) sendToUser(msg *WSMessage) {
	for c := range h.clients[msg.ReceiverID] {
		select {
		case c.send <- msg:
		default:
			h.remove(c)
		}
	}
}
func (h *Hub) broadcastToAll(msg WSMessage) {
	for uid := range h.clients {
		msg.ReceiverID = uid
		h.sendToUser(&msg)
	}
}

func (h *Hub) broadcastPresence(userID, event string) {
	h.broadcastToAll(WSMessage{Event: event, SenderID: userID})
}
func extractMessage(data json.RawMessage) (string, string, error) {
	var p struct {
		Text            string `json:"text"`
		Content         string `json:"content"`
		ClientMessageID string `json:"client_message_id"`
		MessageID       string `json:"message_id"` // legacy client compatibility
	}
	if json.Unmarshal(data, &p) != nil {
		return "", "", errors.New("invalid message data")
	}
	text := p.Text
	if text == "" {
		text = p.Content
	}
	if text == "" {
		return "", "", errors.New("message text missing")
	}
	if p.ClientMessageID == "" {
		p.ClientMessageID = p.MessageID
	}
	if !validClientID(p.ClientMessageID) {
		return "", "", errors.New("invalid client message id")
	}
	if len(text) > 64<<10 {
		return "", "", errors.New("message too large")
	}
	return text, p.ClientMessageID, nil
}
func extractMessageText(data json.RawMessage) (string, error) {
	t, _, e := extractMessage(data)
	return t, e
}
