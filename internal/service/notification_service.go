package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type NotificationService struct {
	repo      repository.NotificationRepository
	publisher func(string, string, any)
}

func NewNotificationService(repo repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) SetPublisher(publisher func(string, string, any)) {
	s.publisher = publisher
}

func (s *NotificationService) PublishCreated(notifications []model.Notification) {
	if s.publisher == nil {
		return
	}
	for _, notification := range notifications {
		s.publisher(notification.RecipientID, "notification.created", notification)
	}
}

func (s *NotificationService) List(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, err := actor(r)
	if err != nil {
		return 401, nil, err
	}
	limit, offset := 30, 0
	if value, parseErr := strconv.Atoi(r.URL.Query().Get("limit")); parseErr == nil && value > 0 && value <= 100 {
		limit = value
	}
	if value, parseErr := strconv.Atoi(r.URL.Query().Get("offset")); parseErr == nil && value >= 0 {
		offset = value
	}
	items, err := s.repo.List(r.Context(), uid, limit, offset)
	if err != nil {
		return 500, nil, err
	}
	return 200, utils.SuccessResponse(map[string]any{"notifications": items, "limit": limit, "offset": offset}), nil
}

func (s *NotificationService) Unread(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, err := actor(r)
	if err != nil {
		return 401, nil, err
	}
	count, err := s.repo.UnreadCount(r.Context(), uid)
	if err != nil {
		return 500, nil, err
	}
	return 200, utils.SuccessResponse(map[string]int{"unread": count}), nil
}

func (s *NotificationService) Read(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, err := actor(r)
	if err != nil {
		return 401, nil, err
	}
	id := chi.URLParam(r, "notificationID")
	if uuid.Validate(id) != nil {
		return 400, nil, errs.ErrValidation
	}
	n, err := s.repo.MarkRead(r.Context(), id, uid, time.Now().UTC())
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return 404, nil, err
		}
		return 500, nil, err
	}
	if s.publisher != nil {
		s.publisher(uid, "notification.read", n)
	}
	return 200, utils.SuccessResponse(n), nil
}

func (s *NotificationService) ReadAll(_ http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, err := actor(r)
	if err != nil {
		return 401, nil, err
	}
	items, err := s.repo.MarkAllRead(r.Context(), uid, time.Now().UTC())
	if err != nil {
		return 500, nil, err
	}
	if s.publisher != nil && len(items) > 0 {
		s.publisher(uid, "notification.read", map[string]any{"all": true, "read_at": time.Now().UTC()})
	}
	return 200, utils.SuccessResponse(map[string]int{"updated": len(items)}), nil
}

func (s *NotificationService) Preferences(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, err := actor(r)
	if err != nil {
		return 401, nil, err
	}
	if r.Method == http.MethodGet {
		p, err := s.repo.GetPreferences(r.Context(), uid)
		if err != nil {
			return 500, nil, err
		}
		return 200, utils.SuccessResponse(p), nil
	}
	var patch struct {
		Message *bool `json:"message_enabled"`
		Reply   *bool `json:"reply_enabled"`
		Mention *bool `json:"mention_enabled"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&patch) != nil || (patch.Message == nil && patch.Reply == nil && patch.Mention == nil) {
		return 400, nil, errs.ErrValidation
	}
	p, err := s.repo.UpdatePreferences(r.Context(), uid, patch.Message, patch.Reply, patch.Mention)
	if err != nil {
		return 500, nil, err
	}
	return 200, utils.SuccessResponse(p), nil
}
