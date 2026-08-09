package service

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/google/uuid"
)

type MessageService interface {
	CreateMessage(ctx context.Context, senderID, receiverID, body string, isGroup bool) (*model.Message, error)
	GetConversation(ctx context.Context, userID, otherUserID string, limit, offset int) (model.Messages, error)
	GetMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type MessageServiceImpl struct {
	messageRepo repository.MessageRepository
	friendRepo  repository.FriendRepository
	blockRepo   repository.BlockRepository
}

func NewMessageServiceImpl(messageRepo repository.MessageRepository, friendRepo repository.FriendRepository, blockRepo repository.BlockRepository) *MessageServiceImpl {
	return &MessageServiceImpl{messageRepo: messageRepo, friendRepo: friendRepo, blockRepo: blockRepo}
}

func (s *MessageServiceImpl) CreateMessage(ctx context.Context, senderID, receiverID, body string, isGroup bool) (*model.Message, error) {
	body = strings.TrimSpace(body)
	if senderID == "" || receiverID == "" || body == "" {
		return nil, errs.ErrBadRequest
	}

	if !isGroup {
		if s.friendRepo == nil || s.blockRepo == nil {
			return nil, errs.ErrInternal
		}

		blocked, err := s.blockRepo.IsBlocked(ctx, senderID, receiverID)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, errs.ErrBlockedRelationship
		}

		areFriends, err := s.friendRepo.AreFriends(ctx, senderID, receiverID)
		if err != nil {
			return nil, err
		}
		if !areFriends {
			return nil, errs.ErrForbidden
		}
	}

	now := time.Now().UTC()
	msg := &model.Message{
		ID:         uuid.New().String(),
		SenderID:   senderID,
		ReceiverID: receiverID,
		Body:       body,
		IsGroup:    isGroup,
		CreatedAt:  now,
		ModifiedAt: now,
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
		return http.StatusInternalServerError, nil, err
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
