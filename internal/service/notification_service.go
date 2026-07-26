package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/google/uuid"
)

type NotificationService interface {
	CreateNotification(ctx context.Context, userID string, notifType model.NotificationType, title, body string, senderID, refID string) (*model.Notification, error)
	CreateFriendRequestNotification(ctx context.Context, receiverID, senderID, requestID string) (*model.Notification, error)
	CreateFriendAcceptedNotification(ctx context.Context, receiverID, acceptorID, requestID string) (*model.Notification, error)
	CreateFriendRejectedNotification(ctx context.Context, receiverID, rejecterID, requestID string) (*model.Notification, error)
	CreateNewMessageNotification(ctx context.Context, receiverID, senderID, messageID, messagePreview string) (*model.Notification, error)
	CreateUserOnlineNotification(ctx context.Context, userID string) error
	CreateUserOfflineNotification(ctx context.Context, userID string) error
	GetNotifications(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	MarkAsRead(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	MarkAllAsRead(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	DeleteNotification(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetUnreadCount(ctx context.Context, userID string) (int, error)
	GetNotificationDTO(n *model.Notification) *model.NotificationDTO
	GetUserByID(ctx context.Context, userID string) (*model.User, error)
}

type NotificationServiceImpl struct {
	notificationRepo repository.NotificationRepository
	userRepo         repository.UserRepository
	friendRepo       repository.FriendRepository
}

func NewNotificationServiceImpl(
	notificationRepo repository.NotificationRepository,
	userRepo repository.UserRepository,
	friendRepo repository.FriendRepository,
) *NotificationServiceImpl {
	return &NotificationServiceImpl{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
		friendRepo:       friendRepo,
	}
}

func (s *NotificationServiceImpl) CreateNotification(ctx context.Context, userID string, notifType model.NotificationType, title, body string, senderID, refID string) (*model.Notification, error) {
	notification := &model.Notification{
		ID:          uuid.New().String(),
		UserID:      userID,
		Type:        notifType,
		Title:       title,
		Body:        body,
		SenderID:    senderID,
		ReferenceID: refID,
		IsRead:      false,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.notificationRepo.CreateNotification(ctx, notification); err != nil {
		return nil, err
	}

	return notification, nil
}

// CreateFriendRequestNotification creates a notification when a friend request is sent
func (s *NotificationServiceImpl) CreateFriendRequestNotification(ctx context.Context, receiverID, senderID, requestID string) (*model.Notification, error) {
	sender, err := s.userRepo.GetByID(ctx, senderID)
	if err != nil || sender == nil {
		sender = &model.User{ID: senderID, Username: "Unknown User"}
	}

	title := "New Friend Request"
	body := sender.Username + " wants to be your friend"

	return s.CreateNotification(ctx, receiverID, model.NotificationFriendRequest, title, body, senderID, requestID)
}

// CreateFriendAcceptedNotification creates a notification when a friend request is accepted
func (s *NotificationServiceImpl) CreateFriendAcceptedNotification(ctx context.Context, receiverID, acceptorID, requestID string) (*model.Notification, error) {
	acceptor, err := s.userRepo.GetByID(ctx, acceptorID)
	if err != nil || acceptor == nil {
		acceptor = &model.User{ID: acceptorID, Username: "Unknown User"}
	}

	title := "Friend Request Accepted"
	body := acceptor.Username + " accepted your friend request"

	return s.CreateNotification(ctx, receiverID, model.NotificationFriendAccepted, title, body, acceptorID, requestID)
}

// CreateFriendRejectedNotification creates a notification when a friend request is rejected
func (s *NotificationServiceImpl) CreateFriendRejectedNotification(ctx context.Context, receiverID, rejecterID, requestID string) (*model.Notification, error) {
	rejecter, err := s.userRepo.GetByID(ctx, rejecterID)
	if err != nil || rejecter == nil {
		rejecter = &model.User{ID: rejecterID, Username: "Unknown User"}
	}

	title := "Friend Request Rejected"
	body := rejecter.Username + " rejected your friend request"

	return s.CreateNotification(ctx, receiverID, model.NotificationFriendRejected, title, body, rejecterID, requestID)
}

// CreateNewMessageNotification creates a notification for a new message when recipient is offline
func (s *NotificationServiceImpl) CreateNewMessageNotification(ctx context.Context, receiverID, senderID, messageID, messagePreview string) (*model.Notification, error) {
	sender, err := s.userRepo.GetByID(ctx, senderID)
	if err != nil || sender == nil {
		sender = &model.User{ID: senderID, Username: "Unknown User"}
	}

	title := "New Message"
	body := sender.Username + ": " + messagePreview
	if len(body) > 100 {
		body = body[:97] + "..."
	}

	return s.CreateNotification(ctx, receiverID, model.NotificationNewMessage, title, body, senderID, messageID)
}

// CreateUserOnlineNotification creates notifications for all friends when a user comes online
func (s *NotificationServiceImpl) CreateUserOnlineNotification(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		user = &model.User{ID: userID, Username: "A user"}
	}

	friendIDs, err := s.friendRepo.GetAllFriendIDs(ctx, userID)
	if err != nil {
		return err
	}

	title := "Friend Online"
	body := user.Username + " came online"

	for _, friendID := range friendIDs {
		_, err := s.CreateNotification(ctx, friendID, model.NotificationUserOnline, title, body, userID, "")
		if err != nil {
			// Log error but continue with other friends
			continue
		}
	}

	return nil
}

// CreateUserOfflineNotification creates notifications for all friends when a user goes offline
func (s *NotificationServiceImpl) CreateUserOfflineNotification(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		user = &model.User{ID: userID, Username: "A user"}
	}

	friendIDs, err := s.friendRepo.GetAllFriendIDs(ctx, userID)
	if err != nil {
		return err
	}

	title := "Friend Offline"
	body := user.Username + " went offline"

	for _, friendID := range friendIDs {
		_, err := s.CreateNotification(ctx, friendID, model.NotificationUserOffline, title, body, userID, "")
		if err != nil {
			// Log error but continue with other friends
			continue
		}
	}

	return nil
}

func (s *NotificationServiceImpl) GetNotifications(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
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

	notifications, err := s.notificationRepo.GetNotificationsForUser(r.Context(), userID, limit, offset)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if notifications == nil {
		notifications = model.Notifications{}
	}

	unreadCount, err := s.notificationRepo.GetUnreadCount(r.Context(), userID)
	if err != nil {
		unreadCount = 0
	}

	responseData := map[string]any{
		"notifications": notifications,
		"unread_count":  unreadCount,
		"limit":         limit,
		"offset":        offset,
	}
	return http.StatusOK, utils.SuccessResponse(responseData), nil
}

func (s *NotificationServiceImpl) MarkAsRead(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	var body struct {
		NotificationID string `json:"notification_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.NotificationID == "" {
		return http.StatusBadRequest, nil, errs.ErrNotFound
	}

	if err := s.notificationRepo.MarkAsRead(r.Context(), body.NotificationID, userID); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, utils.SuccessResponse(map[string]any{}), nil
}

func (s *NotificationServiceImpl) MarkAllAsRead(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	if err := s.notificationRepo.MarkAllAsRead(r.Context(), userID); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, utils.SuccessResponse(map[string]any{}), nil
}

func (s *NotificationServiceImpl) DeleteNotification(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	var body struct {
		NotificationID string `json:"notification_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	if body.NotificationID == "" {
		return http.StatusBadRequest, nil, errs.ErrNotFound
	}

	if err := s.notificationRepo.DeleteNotification(r.Context(), body.NotificationID, userID); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, utils.SuccessResponse(map[string]any{}), nil
}

func (s *NotificationServiceImpl) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	return s.notificationRepo.GetUnreadCount(ctx, userID)
}

func (s *NotificationServiceImpl) GetNotificationDTO(n *model.Notification) *model.NotificationDTO {
	return n.ToDTO()
}

func (s *NotificationServiceImpl) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}