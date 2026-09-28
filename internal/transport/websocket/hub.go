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
	clients         map[string]map[*Client]bool
	rooms           map[string]*Room
	register        chan *Client
	unregister      chan *Client
	incoming        chan *WSMessage
	results         chan *deliveryResult
	messageService  service.MessageService
	quit            chan struct{}
	stopOnce        chan struct{}
	activeSession   func(context.Context, string, string) bool
	presenceService service.PresenceService
	presenceResults chan presenceResult
	external        chan *WSMessage
}

type presenceResult struct {
	client    *Client
	userID    string
	event     string
	timestamp time.Time
	audience  []string
	online    map[string]bool
}

type deliveryResult struct {
	actor       string
	message     *WSMessage
	recipients  []string
	correlation sendCorrelation
	err         error
}

type sendCorrelation struct {
	ClientMessageID string `json:"client_message_id,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
}

func (h *Hub) sendResult(result *deliveryResult) {
	select {
	case h.results <- result:
	case <-h.quit:
	}
}

func NewHub(msgService service.MessageService, presence ...service.PresenceService) *Hub {
	h := &Hub{clients: make(map[string]map[*Client]bool), rooms: make(map[string]*Room), register: make(chan *Client), unregister: make(chan *Client), incoming: make(chan *WSMessage), results: make(chan *deliveryResult, 64), presenceResults: make(chan presenceResult, 64), external: make(chan *WSMessage, 128), messageService: msgService, quit: make(chan struct{}), stopOnce: make(chan struct{}, 1)}
	if len(presence) > 0 {
		h.presenceService = presence[0]
	}
	return h
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
			now := time.Now().UTC()
			wasOffline := len(h.clients[c.userID]) == 0
			if h.clients[c.userID] == nil {
				h.clients[c.userID] = make(map[*Client]bool)
			}
			h.clients[c.userID][c] = true
			if wasOffline {
				h.resolvePresence(nil, c.userID, EventOnline, now, nil)
			}
			h.resolvePresence(c, c.userID, EventPresenceSnapshot, now, h.onlineUsers())
		case c := <-h.unregister:
			wasOnline := len(h.clients[c.userID]) > 0
			h.remove(c)
			if wasOnline && len(h.clients[c.userID]) == 0 {
				h.resolvePresence(nil, c.userID, EventOffline, time.Now().UTC(), nil)
			}
		case result := <-h.presenceResults:
			h.deliverPresence(result)
		case event := <-h.external:
			h.sendToUser(event)
		case msg := <-h.incoming:
			h.dispatch(msg) // never performs database work on this loop
		case result := <-h.results:
			if result.err != nil {
				h.sendDomainErrorWithCorrelation(result.actor, result.err, result.correlation)
				continue
			}
			if result.message.ReceiverType == ReceiverGroup {
				h.sendToRecipients(result.message, result.recipients)
			} else {
				h.sendToUser(result.message)
			}
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
			if result.message.ReceiverType != ReceiverGroup && result.actor != result.message.ReceiverID {
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

func (h *Hub) Publish(userID, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	message := &WSMessage{Event: event, ReceiverID: userID, ReceiverType: ReceiverUser, Data: data}
	select {
	case h.external <- message:
	case <-h.quit:
	}
}

func (h *Hub) onlineUsers() map[string]bool {
	online := make(map[string]bool, len(h.clients))
	for id, clients := range h.clients {
		online[id] = len(clients) > 0
	}
	return online
}

func (h *Hub) resolvePresence(client *Client, userID, event string, timestamp time.Time, online map[string]bool) {
	if h.presenceService == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
		defer cancel()
		audience, err := h.presenceService.Audience(ctx, userID)
		if err != nil {
			return
		}
		result := presenceResult{client: client, userID: userID, event: event, timestamp: timestamp, audience: audience, online: online}
		select {
		case h.presenceResults <- result:
		case <-h.quit:
		}
	}()
}

func (h *Hub) deliverPresence(result presenceResult) {
	if result.event == EventPresenceSnapshot {
		entries := make([]map[string]any, 0, len(result.audience))
		for _, id := range result.audience {
			entries = append(entries, map[string]any{"user_id": id, "online": result.online[id]})
		}
		data, _ := json.Marshal(map[string]any{"timestamp": result.timestamp.Format(time.RFC3339Nano), "users": entries})
		h.sendToClient(result.client, &WSMessage{Event: EventPresenceSnapshot, Data: data})
		return
	}
	data, _ := json.Marshal(map[string]any{"user_id": result.userID, "online": result.event == EventOnline, "timestamp": result.timestamp.Format(time.RFC3339Nano)})
	for _, id := range result.audience {
		h.sendToUser(&WSMessage{Event: result.event, SenderID: result.userID, ReceiverID: id, ReceiverType: ReceiverUser, Data: data})
	}
}

func (h *Hub) sendToClient(client *Client, msg *WSMessage) {
	if client == nil {
		return
	}
	if clients := h.clients[client.userID]; !clients[client] {
		return
	}
	select {
	case client.send <- msg:
	default:
		h.remove(client)
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
	correlation := messageCorrelation(msg.Data)
	if err := msg.Validate(); err != nil {
		h.sendErrorToUserWithCorrelation(actor, err.Error(), "invalid_event", correlation)
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
		if msg.ReceiverType == ReceiverGroup {
			recipients, err := h.realtimeRecipients(actor, p.ConversationID)
			if err != nil {
				h.sendDomainError(actor, err)
				return
			}
			msg.Data, _ = json.Marshal(map[string]any{"conversation_id": p.ConversationID, "state": p.State, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
			h.sendToRecipients(msg, recipients)
		} else {
			msg.Data, _ = json.Marshal(map[string]any{"conversation_id": p.ConversationID, "state": p.State, "timestamp": time.Now().UTC().Format(time.RFC3339Nano)})
			h.sendToUser(msg)
		}
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
			h.sendResult(&deliveryResult{actor: actor, correlation: correlation, err: err})
			return
		}
		var persisted *model.Message
		var recipients []string
		var mentionPayload struct {
			Mentions []model.MessageMention `json:"mentions"`
		}
		_ = json.Unmarshal(msg.Data, &mentionPayload)
		if msg.ReceiverType == ReceiverGroup {
			var p struct {
				ConversationID string `json:"conversation_id"`
			}
			if json.Unmarshal(msg.Data, &p) != nil {
				h.sendResult(&deliveryResult{actor: actor, correlation: correlation, err: errs.ErrValidation})
				return
			}
			s, ok := h.messageService.(interface {
				CreateConversationMessageWithMentions(context.Context, string, string, string, string, string, []model.MessageMention) (*model.Message, []string, error)
			})
			if !ok {
				legacy, legacyOK := h.messageService.(interface {
					CreateConversationMessage(context.Context, string, string, string, string, string) (*model.Message, []string, error)
				})
				if !legacyOK || len(mentionPayload.Mentions) > 0 {
					h.sendResult(&deliveryResult{actor: actor, correlation: correlation, err: errs.ErrInternal})
					return
				}
				persisted, recipients, err = legacy.CreateConversationMessage(ctx, actor, p.ConversationID, text, clientID, "")
			} else {
				persisted, recipients, err = s.CreateConversationMessageWithMentions(ctx, actor, p.ConversationID, text, clientID, "", mentionPayload.Mentions)
			}
		} else if s, ok := h.messageService.(interface {
			CreateConversationMessageWithMentions(context.Context, string, string, string, string, string, []model.MessageMention) (*model.Message, []string, error)
		}); ok {
			var p struct {
				ConversationID string `json:"conversation_id"`
			}
			_ = json.Unmarshal(msg.Data, &p)
			persisted, recipients, err = s.CreateConversationMessageWithMentions(ctx, actor, p.ConversationID, text, clientID, "", mentionPayload.Mentions)
		} else if s, ok := h.messageService.(interface {
			CreateMessageWithClientID(context.Context, string, string, string, string, bool) (*model.Message, error)
		}); ok {
			persisted, err = s.CreateMessageWithClientID(ctx, actor, msg.ReceiverID, text, clientID, false)
		} else {
			persisted, err = h.messageService.CreateMessage(ctx, actor, msg.ReceiverID, text, false)
		}
		if err != nil {
			h.sendResult(&deliveryResult{actor: actor, correlation: correlation, err: err})
			return
		}
		if persisted.IsGroup {
			recipients, err = h.realtimeRecipients(actor, persisted.ConversationID)
			if err != nil {
				h.sendResult(&deliveryResult{actor: actor, correlation: correlation, err: err})
				return
			}
		}
		data, _ := json.Marshal(map[string]any{"message_id": persisted.ID, "client_message_id": clientID, "content": persisted.Body, "conversation_id": persisted.ConversationID, "timestamp": persisted.CreatedAt.Format(time.RFC3339Nano), "mentions": persisted.Mentions})
		h.sendResult(&deliveryResult{actor: actor, recipients: recipients, message: &WSMessage{Event: EventMessage, SenderID: actor, ReceiverID: persisted.ReceiverID, ReceiverType: msg.ReceiverType, Data: data}})
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
	correlation := messageCorrelation(msg.Data)
	if err := msg.Validate(); err != nil {
		h.sendErrorToUserWithCorrelation(actor, err.Error(), "invalid_event", correlation)
		return
	}
	if msg.Event != EventMessage {
		h.dispatchMutationSync(msg, actor)
		return
	}
	text, cid, err := extractMessage(msg.Data)
	if err != nil {
		h.sendErrorToUserWithCorrelation(actor, err.Error(), "invalid_event", correlation)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()
	var persisted *model.Message
	var recipients []string
	var mentionPayload struct {
		Mentions []model.MessageMention `json:"mentions"`
	}
	_ = json.Unmarshal(msg.Data, &mentionPayload)
	if msg.ReceiverType == ReceiverGroup {
		var p struct {
			ConversationID string `json:"conversation_id"`
		}
		if json.Unmarshal(msg.Data, &p) != nil {
			h.sendErrorToUser(actor, "invalid event data", "invalid_event")
			return
		}
		s, ok := h.messageService.(interface {
			CreateConversationMessageWithMentions(context.Context, string, string, string, string, string, []model.MessageMention) (*model.Message, []string, error)
		})
		if !ok {
			legacy, legacyOK := h.messageService.(interface {
				CreateConversationMessage(context.Context, string, string, string, string, string) (*model.Message, []string, error)
			})
			if !legacyOK || len(mentionPayload.Mentions) > 0 {
				h.sendErrorToUserWithCorrelation(actor, "event is not supported", "unsupported_event", correlation)
				return
			}
			persisted, recipients, err = legacy.CreateConversationMessage(ctx, actor, p.ConversationID, text, cid, "")
		} else {
			persisted, recipients, err = s.CreateConversationMessageWithMentions(ctx, actor, p.ConversationID, text, cid, "", mentionPayload.Mentions)
		}
	} else if s, ok := h.messageService.(interface {
		CreateConversationMessageWithMentions(context.Context, string, string, string, string, string, []model.MessageMention) (*model.Message, []string, error)
	}); ok {
		var p struct {
			ConversationID string `json:"conversation_id"`
		}
		_ = json.Unmarshal(msg.Data, &p)
		persisted, recipients, err = s.CreateConversationMessageWithMentions(ctx, actor, p.ConversationID, text, cid, "", mentionPayload.Mentions)
	} else if s, ok := h.messageService.(interface {
		CreateMessageWithClientID(context.Context, string, string, string, string, bool) (*model.Message, error)
	}); ok {
		persisted, err = s.CreateMessageWithClientID(ctx, actor, msg.ReceiverID, text, cid, false)
	} else {
		persisted, err = h.messageService.CreateMessage(ctx, actor, msg.ReceiverID, text, false)
	}
	if err != nil {
		h.sendDomainErrorWithCorrelation(actor, err, correlation)
		return
	}
	if persisted.IsGroup {
		recipients, err = h.realtimeRecipients(actor, persisted.ConversationID)
		if err != nil {
			h.sendDomainErrorWithCorrelation(actor, err, correlation)
			return
		}
	}
	data, _ := json.Marshal(map[string]any{"message_id": persisted.ID, "client_message_id": cid, "content": persisted.Body, "conversation_id": persisted.ConversationID, "timestamp": persisted.CreatedAt.Format(time.RFC3339Nano), "mentions": persisted.Mentions})
	out := &WSMessage{Event: EventMessage, SenderID: actor, ReceiverID: persisted.ReceiverID, ReceiverType: msg.ReceiverType, Data: data}
	if msg.ReceiverType == ReceiverGroup {
		h.sendToRecipients(out, recipients)
	} else {
		h.sendToUser(out)
	}
	ack, _ := json.Marshal(map[string]any{"server_id": persisted.ID, "client_message_id": cid, "status": "sent"})
	h.sendToUser(&WSMessage{Event: EventAck, ReceiverID: actor, ReceiverType: ReceiverUser, Data: ack})
	if msg.ReceiverType != ReceiverGroup && actor != persisted.ReceiverID {
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
			result, err := s.HandleRealtimeResult(ctx, actor, service.RealtimeEvent{Event: msg.Event, ConversationID: p.ConversationID, MessageID: p.MessageID, Content: p.Content, Reaction: p.Reaction, ClientMessageID: p.ClientMessageID, ReceiverID: msg.ReceiverID, Mentions: p.Mentions})
			if err != nil {
				h.sendResult(&deliveryResult{actor: actor, err: err})
				return
			}
			data, _ := json.Marshal(map[string]any{"message_id": result.MessageID, "conversation_id": result.ConversationID, "content": result.Content, "server_id": result.ServerID, "client_message_id": result.ClientMessageID, "reaction": result.Reaction, "reactions": result.Reactions, "mentions": result.Mentions})
			var recipients []string
			if msg.ReceiverType == ReceiverGroup {
				recipients, err = h.realtimeRecipients(actor, result.ConversationID)
				if err != nil {
					h.sendResult(&deliveryResult{actor: actor, err: err})
					return
				}
				recipients = append(recipients, actor)
			}
			h.sendResult(&deliveryResult{actor: actor, recipients: recipients, message: &WSMessage{Event: result.Event, SenderID: actor, ReceiverID: result.ReceiverID, ReceiverType: msg.ReceiverType, Data: data}})
		}()
		return
	}
	// Legacy persistence-only implementations cannot provide the canonical
	// target/message identity required for safe realtime mutation delivery.
	h.sendErrorToUser(actor, "event is not supported", "unsupported_event")
	return
}

type mutationData struct {
	ConversationID  string                 `json:"conversation_id"`
	MessageID       string                 `json:"message_id"`
	Content         string                 `json:"content"`
	Reaction        string                 `json:"reaction"`
	ClientMessageID string                 `json:"client_message_id"`
	Mentions        []model.MessageMention `json:"mentions"`
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
	result, err := persist(ctx, actor, service.RealtimeEvent{Event: msg.Event, ConversationID: p.ConversationID, MessageID: p.MessageID, Content: p.Content, Reaction: p.Reaction, ClientMessageID: p.ClientMessageID, ReceiverID: msg.ReceiverID, Mentions: p.Mentions})
	if err != nil {
		h.sendDomainError(actor, err)
		return
	}
	data, _ := json.Marshal(map[string]any{"message_id": result.MessageID, "conversation_id": result.ConversationID, "content": result.Content, "server_id": result.ServerID, "client_message_id": result.ClientMessageID, "reaction": result.Reaction, "reactions": result.Reactions, "mentions": result.Mentions})
	out := &WSMessage{Event: result.Event, SenderID: actor, ReceiverID: result.ReceiverID, ReceiverType: ReceiverUser, Data: data}
	if msg.ReceiverType == ReceiverGroup {
		out.ReceiverType = ReceiverGroup
		recipients, recipientErr := h.realtimeRecipients(actor, result.ConversationID)
		if recipientErr != nil {
			h.sendDomainError(actor, recipientErr)
			return
		}
		h.sendToRecipients(out, append(recipients, actor))
	} else {
		h.sendToUser(out)
	}
	if actor != result.ReceiverID {
		if msg.ReceiverType != ReceiverGroup {
			copy := *out
			copy.ReceiverID = actor
			h.sendToUser(&copy)
		}
	}
}

func (h *Hub) sendErrorToUser(userID, message, code string) {
	h.sendErrorToUserWithCorrelation(userID, message, code, sendCorrelation{})
}
func (h *Hub) sendErrorToUserWithCorrelation(userID, message, code string, correlation sendCorrelation) {
	data, _ := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		sendCorrelation
	}{Code: code, Message: message, sendCorrelation: correlation})
	h.sendToUser(&WSMessage{Event: EventError, ReceiverID: userID, ReceiverType: ReceiverUser, Data: data})
}
func (h *Hub) sendDomainError(userID string, err error) {
	h.sendDomainErrorWithCorrelation(userID, err, sendCorrelation{})
}
func (h *Hub) sendDomainErrorWithCorrelation(userID string, err error, correlation sendCorrelation) {
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
	data, _ := json.Marshal(struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable bool   `json:"retryable"`
		sendCorrelation
	}{Code: code, Message: message, Retryable: code == "internal_error", sendCorrelation: correlation})
	h.sendToUser(&WSMessage{Event: EventError, ReceiverID: userID, ReceiverType: ReceiverUser, Data: data})
}

func messageCorrelation(data json.RawMessage) sendCorrelation {
	var correlation sendCorrelation
	if json.Unmarshal(data, &correlation) != nil {
		return sendCorrelation{}
	}
	if len(correlation.ClientMessageID) > 128 {
		correlation.ClientMessageID = ""
	}
	if len(correlation.ConversationID) > 128 {
		correlation.ConversationID = ""
	}
	return correlation
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
func (h *Hub) sendToRecipients(msg *WSMessage, recipients []string) {
	for _, uid := range recipients {
		copy := *msg
		copy.ReceiverID = uid
		h.sendToUser(&copy)
	}
}
func (h *Hub) realtimeRecipients(actor, conversationID string) ([]string, error) {
	s, ok := h.messageService.(interface {
		RealtimeRecipients(context.Context, string, string) ([]string, error)
	})
	if !ok {
		return nil, errs.ErrInternal
	}
	ctx, cancel := context.WithTimeout(context.Background(), persistenceTimeout)
	defer cancel()
	return s.RealtimeRecipients(ctx, actor, conversationID)
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
