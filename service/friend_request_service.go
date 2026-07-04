package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/transport/middleware"
	"github.com/google/uuid"
)

type FriendRequestService interface {
	CreateRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	AcceptRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	RejectRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	CancelRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetAllRequests(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetPendingRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type FriendRequestServiceImpl struct {
	repo       repository.FriendRequestRepository
	friendRepo repository.FriendRepository
	blockRepo  repository.BlockRepository
}

func FriendRequestServiceInit(repo repository.FriendRequestRepository,
	friendRepo repository.FriendRepository,
	blockRepo repository.BlockRepository) *FriendRequestServiceImpl {
	return &FriendRequestServiceImpl{repo: repo, friendRepo: friendRepo, blockRepo: blockRepo}
}

// POST
func (s *FriendRequestServiceImpl) CreateRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {

	var body struct {
		To string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.To == "" {
		return http.StatusBadRequest, nil, errs.ErrNotFound
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	if userID == body.To {
		return http.StatusConflict, nil, errs.ErrSelfAction
	}

	// Already friends → reject
	areFriend, err := s.friendRepo.AreFriends(r.Context(), userID, body.To)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.ErrDatabase
	}
	if areFriend {
		return http.StatusConflict, nil, errs.ErrAlreadyFriends
	}

	// Existing pending request (from → to)
	exist, err := s.repo.GetPendingRequest(r.Context(), userID, body.To)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.ErrInternal
	}
	if exist != nil {
		return http.StatusConflict, nil, errs.ErrConflict
	}

	// Existing pending request (to → from)
	reverse, err := s.repo.GetPendingRequest(r.Context(), body.To, userID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.ErrDatabase
	}
	if reverse != nil {
		return http.StatusConflict, nil, errs.ErrConflict
	}

	//check blocking
	blocked, err := s.blockRepo.IsBlocked(r.Context(), body.To, userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if blocked {
		return http.StatusForbidden, nil, errs.ErrBlockedRelationship
	}

	friendReq := &model.FriendRequest{
		ID:         uuid.NewString(),
		SenderID:   userID,
		ReceiverID: body.To,
		Status:     model.FriendPending,
		CreatedAt:  time.Now(),
	}
	if err := s.repo.CreateRequest(r.Context(), friendReq); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusCreated, nil, nil
}

// post
func (s *FriendRequestServiceImpl) AcceptRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {

	var body struct {
		RequestID  string `json:"request_id"`
		ReceiverID string `json:"received_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.RequestID == "" {
		return http.StatusBadRequest, nil, errs.ErrNotFound
	}

	//TODO: block check , already friend check etc!

	if err := s.repo.AcceptRequest(r.Context(), body.RequestID, body.ReceiverID); err != nil {
		return http.StatusInternalServerError, nil, errs.ErrDatabase
	}

	return http.StatusOK, nil, nil
}

// TODO: if no need remove it
func (s *FriendRequestServiceImpl) GetPendingRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	var body struct {
		SenderID string `json:"sender_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.SenderID == "" {
		return http.StatusBadRequest, nil, errs.ErrBadRequest
	}

	pending, err := s.repo.GetPendingRequest(r.Context(), body.SenderID, userID)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if pending == nil {
		return http.StatusNotFound, nil, errors.New("no pending request found")
	}

	responseData := map[string]any{
		"pending_request": pending,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil
}

func (s *FriendRequestServiceImpl) GetAllRequests(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	data, err := s.repo.GetAllRequests(r.Context(), userID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.ErrDatabase
	}

	respData := map[string]any{
		"requests": data,
	}

	return http.StatusOK, utils.SuccessResponse(respData), nil

}

func (s *FriendRequestServiceImpl) RejectRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var body struct {
		RequestID  string `json:"request_id"`
		ReceiverID string `json:"receiver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.RequestID == "" {
		return http.StatusBadRequest, nil, errs.ErrBadRequest
	}

	// userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	// if !ok || userID == "" {
	// 	return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	// }

	if err := s.repo.RejectRequest(r.Context(), body.RequestID, body.ReceiverID); err != nil {
		return http.StatusInternalServerError, nil, errs.ErrDatabase
	}

	return http.StatusOK, nil, nil
}

func (s *FriendRequestServiceImpl) CancelRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var body struct {
		RequestID string `json:"request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.RequestID == "" {
		return http.StatusBadRequest, nil, errs.ErrBadRequest
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	if err := s.repo.CancelRequest(r.Context(), body.RequestID, userID); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil
}
