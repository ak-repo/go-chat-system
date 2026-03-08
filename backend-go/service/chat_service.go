package service

import (
	"encoding/json"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/transport/middleware"
)

type ChatService interface {
	ListMyChats(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetOrCreateDM(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type ChatServiceImpl struct {
	chatRepo repository.ChatRepository
}

func NewChatServiceImpl(chatRepo repository.ChatRepository) *ChatServiceImpl {
	return &ChatServiceImpl{chatRepo: chatRepo}
}

func (s *ChatServiceImpl) ListMyChats(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	chats, err := s.chatRepo.GetUserChats(r.Context(), userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, utils.SuccessResponse(map[string]any{"chats": chats}), nil
}

func (s *ChatServiceImpl) GetOrCreateDM(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	var req struct {
		OtherUserID string `json:"other_user_id"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		return http.StatusBadRequest, nil, err
	}
	if req.OtherUserID == "" {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}
	if req.OtherUserID == userID {
		return http.StatusBadRequest, nil, errs.ErrSelfAction
	}
	chat, err := s.chatRepo.GetOrCreateDMChat(r.Context(), userID, req.OtherUserID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, utils.SuccessResponse(map[string]any{"chat": chat}), nil
}
