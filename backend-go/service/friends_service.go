package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/transport/middleware"
	"github.com/google/uuid"
)

type FriendService interface {
	SendRequest(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error)
	AcceptRequest(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error)
	BlockUser(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error)
	ListFriends(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error)
	GetAllRequests(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error)
	// TODO: list user comming and sented requests
}

type FriendServiceImpl struct {
	friendRepo repository.FriendRepository
	userRepo   repository.UserRepository
}

func NewFriendServiceImpl(repo repository.FriendRepository) *FriendServiceImpl {
	return &FriendServiceImpl{friendRepo: repo}
}

func (s *FriendServiceImpl) GetAllRequests(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error) {

	return http.StatusOK, nil, nil

}

// POST
// TODO: chack give id is valid user
func (s *FriendServiceImpl) SendRequest(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error) {

	var body struct {
		To string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.To == "" {
		return http.StatusBadRequest, nil, errors.New("friend id missing ")
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errors.New("user id missing ")
	}

	if userID == body.To {
		return http.StatusConflict, nil, errors.New("cannot add yourself")
	}

	// Already friends → reject
	areFriends, err := s.friendRepo.AreFriends(r.Context(), userID, body.To)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if areFriends {
		return http.StatusConflict, nil, errors.New("already friends")
	}

	// Existing pending request (from → to)
	existing, err := s.friendRepo.GetPendingRequest(r.Context(), userID, body.To)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if existing != nil {
		return http.StatusConflict, nil, errors.New("request already exists")
	}

	// Existing pending request (to → from)
	reverse, err := s.friendRepo.GetPendingRequest(r.Context(), body.To, userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if reverse != nil {
		return http.StatusConflict, nil, errors.New("incoming request already exists")
	}

	friendReq := &model.FriendRequest{
		ID:         uuid.NewString(),
		SenderID:   userID,
		ReceiverID: body.To,
		Status:     model.FriendPending,
		CreatedAt:  time.Now(),
	}
	if err := s.friendRepo.CreateRequest(r.Context(), friendReq); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}

// post
func (s *FriendServiceImpl) AcceptRequest(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error) {

	var body struct {
		RequestID string `json:"request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.RequestID == "" {
		return http.StatusBadRequest, nil, errors.New("request_id is required")
	}
	s.friendRepo.AcceptRequest(r.Context(), body.RequestID, "123")
	return http.StatusOK, nil, nil
}

// POST
func (s *FriendServiceImpl) BlockUser(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error) {

	var body struct {
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errors.New("user id missing ")
	}

	if userID == body.Target {
		return http.StatusConflict, nil, errors.New("cannot block yourself")
	}

	if err := s.friendRepo.BlockUser(r.Context(), userID, body.Target); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil

}

func (s *FriendServiceImpl) ListFriends(w http.ResponseWriter, r *http.Request) (int, *model.ApiResponse, error) {

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errors.New("user id missing ")
	}

	respObj, err := s.friendRepo.ListFriends(r.Context(), userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	responseData := map[string]any{
		"friends": respObj,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil
}
