package service

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/google/uuid"
)

type UserService interface {
	SearchUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	Register(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	Login(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)

	//TODO: admin actions

}

type UserServiceImpl struct {
	userRepo repository.UserRepository
}

func NewUserServiceImpl(userRepo repository.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: userRepo}
}

func (s *UserServiceImpl) SearchUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
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

func (s *UserServiceImpl) Register(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if !utils.Required(req.Username) ||
		!utils.Required(req.Email) ||
		!utils.Required(req.Password) {

		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {

		return http.StatusInternalServerError, nil, err
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         "user",
		CreatedAt:    time.Now().UTC(),
		ModifiedAt:   time.Now().UTC(),
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"userID":     user.ID,
		"created_at": user.CreatedAt,
	}
	return http.StatusCreated, utils.SuccessResponse(responseData), nil

}

func (s *UserServiceImpl) Login(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if !utils.Required(req.Email) || !utils.Required(req.Password) {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if user == nil {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized

	}

	if !utils.ComparePassword(user.PasswordHash, req.Password) {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	user.PasswordHash = ""

	token, ttl, err := jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"user": &model.UserDTO{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
		"token": token,
		"exp":   ttl,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil

}
