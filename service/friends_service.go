package service

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/transport/middleware"
)

type FriendService interface {
	ListFriends(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type FriendServiceImpl struct {
	friendRepo repository.FriendRepository
	userRepo   repository.UserRepository
}

func NewFriendServiceImpl(repo repository.FriendRepository) *FriendServiceImpl {
	return &FriendServiceImpl{friendRepo: repo}
}

func (s *FriendServiceImpl) ListFriends(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errors.New("user id missing ")
	}

	limit := 20
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

	data, err := s.friendRepo.ListFriends(r.Context(), userID, limit, offset)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"friends": data,
		"limit":   limit,
		"offset":  offset,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil
}
