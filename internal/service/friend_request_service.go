package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/google/uuid"
)

type FriendRequestService interface {
	CreateRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	AcceptRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	RejectRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	CancelRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetAllRequests(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetPendingRequest(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	SetHub(hub interface{ SendFriendRequestNotification(receiverID, requestID, senderID, senderUsername string) })
}

type FriendRequestServiceImpl struct {
	repo                repository.FriendRequestRepository
	friendRepo          repository.FriendRepository
	blockRepo           repository.BlockRepository
	notificationService NotificationService
	hub                 interface {
		SendFriendRequestNotification(receiverID, requestID, senderID, senderUsername string)
	}
}

func FriendRequestServiceInit(repo repository.FriendRequestRepository,
	friendRepo repository.FriendRepository,
	blockRepo repository.BlockRepository,
	notificationService NotificationService) *FriendRequestServiceImpl {
	return &FriendRequestServiceImpl{
		repo:                repo,
		friendRepo:          friendRepo,
		blockRepo:           blockRepo,
		notificationService: notificationService,
	}
}

// SetHub sets the WebSocket hub for real-time notifications
func (s *FriendRequestServiceImpl) SetHub(hub interface{ SendFriendRequestNotification(receiverID, requestID, senderID, senderUsername string) }) {
	s.hub = hub
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
		return http.StatusBadRequest, nil, errs.ErrSelfAction
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

	// Get sender info for WebSocket notification
	sender, _ := s.notificationService.GetUserByID(r.Context(), userID)
	senderUsername := "Someone"
	if sender != nil {
		senderUsername = sender.Username
	}

	// Create notification for the receiver
	if s.notificationService != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := s.notificationService.CreateFriendRequestNotification(ctx, body.To, userID, friendReq.ID)
			if err != nil {
				// Log error but don't fail the request
			}
		}()
	}

	// Send real-time WebSocket notification
	if s.hub != nil {
		go func() {
			s.hub.SendFriendRequestNotification(body.To, friendReq.ID, userID, senderUsername)
		}()
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

	// Create notification for the original sender
	if s.notificationService != nil {
		// Get the friend request to find the original sender
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Find the request to get sender info
			if req, err := s.repo.GetRequestByID(ctx, body.RequestID); err == nil && req != nil {
				_, err := s.notificationService.CreateFriendAcceptedNotification(ctx, req.SenderID, body.ReceiverID, req.ID)
				if err != nil {
					// Log error but don't fail the request
				}
			}
		}()
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

	// Create notification for the original sender
	if s.notificationService != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// Find the request to get sender info
			if req, err := s.repo.GetRequestByID(ctx, body.RequestID); err == nil && req != nil {
				_, err := s.notificationService.CreateFriendRejectedNotification(ctx, req.SenderID, body.ReceiverID, req.ID)
				if err != nil {
					// Log error but don't fail the request
				}
			}
		}()
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
