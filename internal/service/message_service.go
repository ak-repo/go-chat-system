package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type MessageService interface {
	CreateMessage(ctx context.Context, senderID, receiverID, body string, isGroup bool) (*model.Message, error)
	GetConversation(ctx context.Context, userID, otherUserID string, limit, offset int) (model.Messages, error)
	GetMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}
type MessageHTTPService interface {
	History(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Send(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Edit(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Delete(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Delivery(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Read(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Unread(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
}

type RealtimeEvent struct{ Event, ConversationID, MessageID, Content, ReceiverID, Status string }
type RealtimeResult struct{ Event, ConversationID, MessageID, Content, ReceiverID, ServerID string }

// HandleRealtime is the persistence boundary used by the WebSocket transport.
// Actor is supplied by authenticated socket context, never by RealtimeEvent.
func (s *MessageServiceImpl) HandleRealtime(ctx context.Context, actor string, e RealtimeEvent) error {
	_, err := s.HandleRealtimeResult(ctx, actor, e)
	return err
}

func (s *MessageServiceImpl) AuthorizeRealtime(ctx context.Context, actor string, e RealtimeEvent) error {
	if s.conversationRepo == nil || e.ReceiverID == "" {
		return errs.ErrForbidden
	}
	for _, uid := range []string{actor, e.ReceiverID} {
		ok, err := s.conversationRepo.IsMember(ctx, e.ConversationID, uid)
		if err != nil {
			return errs.Wrap("service.MessageService.RealtimeMember", err)
		}
		if !ok {
			return errs.ErrNotMember
		}
	}
	if err := s.authorizeDirect(ctx, actor, e.ReceiverID); err != nil {
		return err
	}
	return nil
}

func (s *MessageServiceImpl) HandleRealtimeResult(ctx context.Context, actor string, e RealtimeEvent) (*RealtimeResult, error) {
	if s.conversationRepo == nil || s.messageRepo == nil {
		return nil, errs.ErrInternal
	}
	ok, err := s.conversationRepo.IsMember(ctx, e.ConversationID, actor)
	if err != nil {
		return nil, errs.Wrap("service.MessageService.RealtimeMember", err)
	}
	if !ok {
		return nil, errs.ErrNotMember
	}
	mr, ok := s.messageRepo.(repository.MessageMutationRepository)
	if !ok {
		return nil, errs.ErrInternal
	}
	target, err := mr.GetMessage(ctx, e.MessageID)
	if err != nil || target == nil || target.ConversationID != e.ConversationID {
		return nil, errs.ErrForbidden
	}
	switch e.Event {
	case "message.edited":
		if target.SenderID != actor {
			return nil, errs.ErrForbidden
		}
		content := strings.TrimSpace(e.Content)
		if content == "" || len(content) > utils.MaxMessageLength {
			return nil, errs.ErrValidation
		}
		if err := mr.EditMessage(ctx, e.MessageID, actor, content); err != nil {
			return nil, err
		}
		return &RealtimeResult{Event: e.Event, ConversationID: target.ConversationID, MessageID: target.ID, Content: content, ReceiverID: target.ReceiverID}, nil
	case "message.deleted":
		if target.SenderID != actor {
			return nil, errs.ErrForbidden
		}
		if err := mr.DeleteMessage(ctx, e.MessageID, actor); err != nil {
			return nil, err
		}
		return &RealtimeResult{Event: e.Event, ConversationID: target.ConversationID, MessageID: target.ID, ReceiverID: target.ReceiverID}, nil
	case "message.delivered":
		if target.ReceiverID != actor {
			return nil, errs.ErrForbidden
		}
		if err := mr.MarkDelivery(ctx, e.MessageID, actor, "delivered", time.Now().UTC()); err != nil {
			return nil, err
		}
		return &RealtimeResult{Event: e.Event, ConversationID: target.ConversationID, MessageID: target.ID, ReceiverID: target.SenderID}, nil
	case "message.read":
		if target.ReceiverID != actor {
			return nil, errs.ErrForbidden
		}
		if err := mr.MarkRead(ctx, e.ConversationID, actor, e.MessageID, time.Now().UTC()); err != nil {
			return nil, err
		}
		return &RealtimeResult{Event: e.Event, ConversationID: target.ConversationID, MessageID: target.ID, ReceiverID: target.SenderID}, nil
	case "message.replied":
		if strings.TrimSpace(e.Content) == "" || len(strings.TrimSpace(e.Content)) > utils.MaxMessageLength || e.ReceiverID == "" {
			return nil, errs.ErrValidation
		}
		target, te := mr.GetMessage(ctx, e.MessageID)
		if te != nil || target == nil || target.ConversationID != e.ConversationID || (target.SenderID != actor && target.ReceiverID != actor) || (target.SenderID == actor && e.ReceiverID != target.ReceiverID) || (target.ReceiverID == actor && e.ReceiverID != target.SenderID) {
			return nil, errs.ErrValidation
		}
		if err := s.authorizeDirect(ctx, actor, e.ReceiverID); err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		reply := &model.Message{ID: uuid.NewString(), SenderID: actor, ReceiverID: e.ReceiverID, Body: strings.TrimSpace(e.Content), ConversationID: e.ConversationID, ReplyToMessageID: e.MessageID, CreatedAt: now, ModifiedAt: now}
		if err := mr.CreateReply(ctx, reply); err != nil {
			return nil, err
		}
		return &RealtimeResult{Event: e.Event, ConversationID: e.ConversationID, MessageID: target.ID, Content: reply.Body, ReceiverID: reply.ReceiverID, ServerID: reply.ID}, nil
	default:
		return nil, errs.ErrBadRequest
	}
}

type MessageServiceImpl struct {
	messageRepo      repository.MessageRepository
	friendRepo       repository.FriendRepository
	blockRepo        repository.BlockRepository
	conversationRepo repository.ConversationRepository
}

func (s *MessageServiceImpl) SetConversationRepository(r repository.ConversationRepository) {
	s.conversationRepo = r
}

func NewMessageServiceImpl(messageRepo repository.MessageRepository, friendRepo repository.FriendRepository, blockRepo repository.BlockRepository) *MessageServiceImpl {
	return &MessageServiceImpl{messageRepo: messageRepo, friendRepo: friendRepo, blockRepo: blockRepo}
}

func (s *MessageServiceImpl) CreateMessage(ctx context.Context, senderID, receiverID, body string, isGroup bool) (*model.Message, error) {
	return s.createMessage(ctx, senderID, receiverID, body, "", isGroup)
}

func (s *MessageServiceImpl) CreateMessageWithClientID(ctx context.Context, senderID, receiverID, body, clientID string, isGroup bool) (*model.Message, error) {
	var conversationID string
	if clientID != "" && s.conversationRepo != nil {
		if c, err := s.conversationRepo.CreateOrGetDirect(ctx, senderID, receiverID); err == nil {
			conversationID = c.ID
			if mr, ok := s.messageRepo.(repository.MessageMutationRepository); ok {
				if old, lookupErr := mr.GetByClientMessageID(ctx, clientID, senderID, c.ID); lookupErr == nil && old != nil {
					return old, nil
				}
			}
		}
	}
	m, err := s.createMessage(ctx, senderID, receiverID, body, clientID, isGroup)
	if err != nil && clientID != "" && conversationID != "" {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if mr, ok := s.messageRepo.(repository.MessageMutationRepository); ok {
				if old, lookupErr := mr.GetByClientMessageID(ctx, clientID, senderID, conversationID); lookupErr == nil && old != nil {
					return old, nil
				}
			}
		}
	}
	return m, err
}

func (s *MessageServiceImpl) createMessage(ctx context.Context, senderID, receiverID, body, clientID string, isGroup bool) (*model.Message, error) {
	body = strings.TrimSpace(body)
	if senderID == "" || receiverID == "" || body == "" {
		return nil, errs.ErrBadRequest
	}
	if len(body) > utils.MaxMessageLength {
		return nil, errs.ErrValidation
	}

	if !isGroup {
		if s.friendRepo == nil || s.blockRepo == nil {
			return nil, errs.ErrInternal
		}

		blocked, err := s.blockRepo.IsBlocked(ctx, senderID, receiverID)
		if err != nil {
			return nil, errs.Wrap("service.MessageService.CreateMessage", err)
		}
		if blocked {
			return nil, errs.ErrBlockedRelationship
		}

		areFriends, err := s.friendRepo.AreFriends(ctx, senderID, receiverID)
		if err != nil {
			return nil, errs.Wrap("service.MessageService.CreateMessage", err)
		}
		if !areFriends {
			return nil, errs.ErrForbidden
		}
	}

	now := time.Now().UTC()
	msg := &model.Message{
		ID:              uuid.New().String(),
		SenderID:        senderID,
		ReceiverID:      receiverID,
		Body:            body,
		ClientMessageID: clientID,
		IsGroup:         isGroup,
		CreatedAt:       now,
		ModifiedAt:      now,
	}
	if s.conversationRepo != nil {
		c, err := s.conversationRepo.CreateOrGetDirect(ctx, senderID, receiverID)
		if err != nil {
			return nil, errs.Wrap("service.MessageService.CreateMessage", err)
		}
		msg.ConversationID = c.ID
	}

	if err := s.messageRepo.CreateMessage(ctx, msg); err != nil {
		return nil, errs.Wrap("service.MessageService.CreateMessage", err)
	}
	return msg, nil
}

func (s *MessageServiceImpl) GetConversation(ctx context.Context, userID, otherUserID string, limit, offset int) (model.Messages, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	messages, err := s.messageRepo.GetMessagesBetweenUsers(ctx, userID, otherUserID, limit, offset)
	if err != nil {
		return nil, errs.Wrap("service.MessageService.GetConversation", err)
	}
	return messages, nil
}

func (s *MessageServiceImpl) authorizeDirect(ctx context.Context, a, b string) error {
	if s.friendRepo == nil || s.blockRepo == nil {
		return errs.ErrInternal
	}
	blocked, err := s.blockRepo.IsBlocked(ctx, a, b)
	if err != nil {
		return errs.Wrap("service.MessageService.authorize", err)
	}
	if blocked {
		return errs.ErrBlockedRelationship
	}
	friends, err := s.friendRepo.AreFriends(ctx, a, b)
	if err != nil {
		return errs.Wrap("service.MessageService.authorize", err)
	}
	if !friends {
		return errs.ErrForbidden
	}
	return nil
}

func (s *MessageServiceImpl) GetMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	otherUserID := r.URL.Query().Get("user_id")
	if otherUserID == "" {
		return http.StatusBadRequest, nil, nil
	}

	userIDVal := r.Context().Value(middleware.UserIDKey)
	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	messages, err := s.GetConversation(r.Context(), userID, otherUserID, limit, offset)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.MessageService.GetMessages", err)
	}
	if messages == nil {
		messages = model.Messages{}
	}

	responseData := map[string]any{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	}
	return http.StatusOK, utils.SuccessResponse(responseData), nil
}

func (s *MessageServiceImpl) History(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	if s.conversationRepo == nil {
		return 500, nil, errs.ErrInternal
	}
	conversationID := chi.URLParam(r, "conversationID")
	if uuid.Validate(conversationID) != nil {
		return 400, nil, errs.ErrValidation
	}
	c, e := s.conversationRepo.Get(r.Context(), conversationID, uid)
	if e != nil {
		return 404, nil, e
	}
	limit, offset := 50, 0
	if v, _ := strconv.Atoi(r.URL.Query().Get("limit")); v > 0 && v <= 100 {
		limit = v
	}
	if v, _ := strconv.Atoi(r.URL.Query().Get("offset")); v >= 0 {
		offset = v
	}
	other := c.UserOneID
	if other == uid {
		other = c.UserTwoID
	}
	if e = s.authorizeDirect(r.Context(), uid, other); e != nil {
		return 403, nil, e
	}
	m, e := s.GetConversation(r.Context(), uid, other, limit, offset)
	if e != nil {
		return 500, nil, e
	}
	if m == nil {
		m = model.Messages{}
	}
	return 200, utils.SuccessResponse(map[string]any{"conversation_id": c.ID, "messages": m, "limit": limit, "offset": offset}), nil
}
func (s *MessageServiceImpl) Send(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		ConversationID   string `json:"conversation_id"`
		Content          string `json:"content"`
		ClientMessageID  string `json:"client_message_id"`
		ReplyToMessageID string `json:"reply_to_message_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&q) != nil {
		return 400, nil, errs.ErrValidation
	}
	if pathID := chi.URLParam(r, "conversationID"); pathID != "" {
		if q.ConversationID != "" && q.ConversationID != pathID {
			return 400, nil, errs.ErrValidation
		}
		q.ConversationID = pathID
	}
	q.Content = strings.TrimSpace(q.Content)
	if q.ConversationID == "" || q.Content == "" || len(q.Content) > utils.MaxMessageLength {
		return 400, nil, errs.ErrValidation
	}
	if uuid.Validate(q.ConversationID) != nil {
		return 400, nil, errs.ErrValidation
	}
	if len(q.ClientMessageID) > 128 || len(q.ReplyToMessageID) > 128 {
		return 400, nil, errs.ErrValidation
	}
	if q.ClientMessageID != "" && uuid.Validate(q.ClientMessageID) != nil {
		return 400, nil, errs.ErrValidation
	}
	if q.ReplyToMessageID != "" && uuid.Validate(q.ReplyToMessageID) != nil {
		return 400, nil, errs.ErrValidation
	}
	c, e := s.conversationRepo.Get(r.Context(), q.ConversationID, uid)
	if e != nil {
		return 404, nil, e
	}
	if mr, ok := s.messageRepo.(repository.MessageMutationRepository); ok && q.ClientMessageID != "" {
		if old, _ := mr.GetByClientMessageID(r.Context(), q.ClientMessageID, uid, q.ConversationID); old != nil {
			return 200, utils.SuccessResponse(old), nil
		}
	}
	other := c.UserOneID
	if other == uid {
		other = c.UserTwoID
	}
	if e = s.authorizeDirect(r.Context(), uid, other); e != nil {
		return 403, nil, e
	}
	m := &model.Message{ID: uuid.NewString(), SenderID: uid, ReceiverID: other, Body: strings.TrimSpace(q.Content), ConversationID: q.ConversationID, ClientMessageID: q.ClientMessageID, ReplyToMessageID: q.ReplyToMessageID, CreatedAt: time.Now().UTC(), ModifiedAt: time.Now().UTC()}
	if q.ReplyToMessageID != "" {
		mr, ok := s.messageRepo.(repository.MessageMutationRepository)
		if !ok {
			return 500, nil, errs.ErrInternal
		}
		reply, re := mr.GetMessage(r.Context(), q.ReplyToMessageID)
		if re != nil || reply == nil || reply.ConversationID != q.ConversationID || (reply.SenderID != uid && reply.SenderID != other) {
			return 400, nil, errs.ErrValidation
		}
	}
	if e = s.messageRepo.CreateMessage(r.Context(), m); e != nil {
		var pgErr *pgconn.PgError
		if q.ClientMessageID != "" && errors.As(e, &pgErr) && pgErr.Code == "23505" {
			if mr, ok := s.messageRepo.(repository.MessageMutationRepository); ok {
				if old, lookupErr := mr.GetByClientMessageID(r.Context(), q.ClientMessageID, uid, q.ConversationID); lookupErr == nil && old != nil {
					return 200, utils.SuccessResponse(old), nil
				}
			}
		}
		return 500, nil, e
	}
	return 201, utils.SuccessResponse(m), nil
}
func (s *MessageServiceImpl) Edit(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		Content string `json:"content"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || strings.TrimSpace(q.Content) == "" || len(strings.TrimSpace(q.Content)) > utils.MaxMessageLength {
		return 400, nil, errs.ErrValidation
	}
	mr, ok := s.messageRepo.(repository.MessageMutationRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	messageID := chi.URLParam(r, "messageID")
	if uuid.Validate(messageID) != nil {
		return 400, nil, errs.ErrValidation
	}
	if e = mr.EditMessage(r.Context(), messageID, uid, strings.TrimSpace(q.Content)); e != nil {
		return 404, nil, e
	}
	return 200, utils.SuccessResponse(map[string]string{"id": messageID, "content": q.Content}), nil
}
func (s *MessageServiceImpl) Delete(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	mr, ok := s.messageRepo.(repository.MessageMutationRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	messageID := chi.URLParam(r, "messageID")
	if uuid.Validate(messageID) != nil {
		return 400, nil, errs.ErrValidation
	}
	if e = mr.DeleteMessage(r.Context(), messageID, uid); e != nil {
		return 404, nil, e
	}
	return 200, utils.SuccessResponse(map[string]string{"id": messageID, "status": "deleted"}), nil
}
func (s *MessageServiceImpl) Delivery(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		MessageID string `json:"message_id"`
		Status    string `json:"status"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || q.MessageID == "" || (q.Status != "delivered" && q.Status != "read") {
		return 400, nil, errs.ErrValidation
	}
	mr, ok := s.messageRepo.(repository.MessageMutationRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	message, e := mr.GetMessage(r.Context(), q.MessageID)
	if e != nil || message == nil || message.ReceiverID != uid {
		return 403, nil, errs.ErrForbidden
	}
	if e = mr.MarkDelivery(r.Context(), q.MessageID, uid, q.Status, time.Now().UTC()); e != nil {
		return 500, nil, e
	}
	return 200, utils.SuccessResponse(map[string]string{"status": q.Status}), nil
}
func (s *MessageServiceImpl) Read(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		MessageID string `json:"message_id"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.MessageID == "" {
		return 400, nil, errs.ErrValidation
	}
	cid := chi.URLParam(r, "conversationID")
	if uuid.Validate(cid) != nil {
		return 400, nil, errs.ErrValidation
	}
	ok, e := s.conversationRepo.IsMember(r.Context(), cid, uid)
	if e != nil || !ok {
		return 403, nil, errs.ErrNotMember
	}
	mr, ok := s.messageRepo.(repository.MessageMutationRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	message, e := mr.GetMessage(r.Context(), q.MessageID)
	if e != nil || message == nil || message.ConversationID != cid || message.ReceiverID != uid {
		return 403, nil, errs.ErrForbidden
	}
	if e = mr.MarkRead(r.Context(), cid, uid, q.MessageID, time.Now().UTC()); e != nil {
		return 500, nil, e
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "read"}), nil
}
func (s *MessageServiceImpl) Unread(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	mr, ok := s.messageRepo.(repository.MessageMutationRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	u, e := mr.UnreadSummary(r.Context(), uid)
	if e != nil {
		return 500, nil, e
	}
	return 200, utils.SuccessResponse(map[string]any{"unread": u}), nil
}
