package service

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/google/uuid"
)

type MessageService interface {
	CreateMessage(ctx context.Context, senderID, receiverID, body string, isGroup bool) (*model.Message, error)
	GetConversation(ctx context.Context, userID, otherUserID string, limit, offset int) (model.Messages, error)
	GetMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type MessageServiceImpl struct {
	messageRepo repository.MessageRepository
}

func NewMessageServiceImpl(messageRepo repository.MessageRepository) *MessageServiceImpl {
	return &MessageServiceImpl{messageRepo: messageRepo}
}

func (s *MessageServiceImpl) CreateMessage(ctx context.Context, senderID, receiverID, body string, isGroup bool) (*model.Message, error) {
	msg := &model.Message{
		ID:         uuid.New().String(),
		SenderID:   senderID,
		ReceiverID: receiverID,
		Body:       body,
		IsGroup:    isGroup,
		CreatedAt:  time.Now().UTC(),
		ModifiedAt: time.Now().UTC(),
	}

	if err := s.messageRepo.CreateMessage(ctx, msg); err != nil {
		return nil, err
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
	return s.messageRepo.GetMessagesBetweenUsers(ctx, userID, otherUserID, limit, offset)
}

func (s *MessageServiceImpl) GetMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	otherUserID := r.URL.Query().Get("user_id")
	if otherUserID == "" {
		return http.StatusBadRequest, nil, nil
	}

	userID := r.Context().Value("userID").(string)

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
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	}
	return http.StatusOK, utils.SuccessResponse(responseData), nil
}
