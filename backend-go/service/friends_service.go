package service

import (
	"errors"
	"net/http"

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

	data, err := s.friendRepo.ListFriends(r.Context(), userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"friends": data,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil
}
