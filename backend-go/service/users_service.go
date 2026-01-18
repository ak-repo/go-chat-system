package service

import (
	"net/http"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
)

type UserService interface {
	SearchUser(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error)
}

type UserServiceImpl struct {
	userRepo repository.UserRepository
}

func NewUserServiceImpl(userRepo repository.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: userRepo}
}

func (s *UserServiceImpl) SearchUser(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error) {
	filter := r.URL.Query().Get("filter")

	respObj, err := s.userRepo.SearchUser(r.Context(), filter)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"users": respObj,
	}
	return http.StatusOK, utils.SuccessResponse(responseData), nil
}
