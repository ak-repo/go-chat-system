package service

import (
	"net/http"
	"strconv"

	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/transport/middleware"
	"github.com/go-chi/chi"
)

type MessageService interface {
	ListMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type MessageServiceImpl struct {
	messageRepo repository.MessageRepository
	chatRepo    repository.ChatRepository
}

func NewMessageServiceImpl(messageRepo repository.MessageRepository, chatRepo repository.ChatRepository) *MessageServiceImpl {
	return &MessageServiceImpl{messageRepo: messageRepo, chatRepo: chatRepo}
}

func (s *MessageServiceImpl) ListMessages(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	chatID := chi.URLParam(r, "chatID")
	if chatID == "" {
		return http.StatusBadRequest, nil, errs.ErrBadRequest
	}
	member, err := s.chatRepo.IsMember(r.Context(), chatID, userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if !member {
		return http.StatusForbidden, nil, errs.ErrForbidden
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	beforeID := r.URL.Query().Get("before")
	messages, err := s.messageRepo.ListByChatID(r.Context(), chatID, limit, beforeID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, utils.SuccessResponse(map[string]any{"messages": messages}), nil
}
