package service

import (
	"encoding/json"
	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
)

type ConversationService interface {
	Create(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	List(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	Get(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
}
type ConversationServiceImpl struct {
	repo    repository.ConversationRepository
	friends repository.FriendRepository
	blocks  repository.BlockRepository
}

func NewConversationService(r repository.ConversationRepository, deps ...any) *ConversationServiceImpl {
	s := &ConversationServiceImpl{repo: r}
	for _, d := range deps {
		switch v := d.(type) {
		case repository.FriendRepository:
			s.friends = v
		case repository.BlockRepository:
			s.blocks = v
		}
	}
	return s
}
func cid(r *http.Request) (string, error) {
	v, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || v == "" {
		return "", errs.ErrUnauthorized
	}
	return v, nil
}
func (s *ConversationServiceImpl) Create(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := cid(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		UserID string `json:"user_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || strings.TrimSpace(q.UserID) == "" || q.UserID == uid {
		return 400, nil, errs.ErrValidation
	}
	if s.friends == nil || s.blocks == nil {
		return 500, nil, errs.ErrInternal
	}
	blocked, e := s.blocks.IsBlocked(r.Context(), uid, q.UserID)
	if e != nil {
		return 500, nil, errs.ErrInternal
	}
	if blocked {
		return 403, nil, errs.ErrBlockedRelationship
	}
	friends, e := s.friends.AreFriends(r.Context(), uid, q.UserID)
	if e != nil {
		return 500, nil, errs.ErrInternal
	}
	if !friends {
		return 403, nil, errs.ErrForbidden
	}
	c, e := s.repo.CreateOrGetDirect(r.Context(), uid, strings.TrimSpace(q.UserID))
	if e != nil {
		return 500, nil, e
	}
	return 200, utils.SuccessResponse(c), nil
}
func (s *ConversationServiceImpl) List(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := cid(r)
	if e != nil {
		return 401, nil, e
	}
	limit, offset := 50, 0
	if v, _ := strconv.Atoi(r.URL.Query().Get("limit")); v > 0 && v <= 100 {
		limit = v
	}
	if v, _ := strconv.Atoi(r.URL.Query().Get("offset")); v >= 0 {
		offset = v
	}
	cs, e := s.repo.List(r.Context(), uid, limit, offset)
	if e != nil {
		return 500, nil, e
	}
	if cs == nil {
		cs = []*model.Conversation{}
	}
	return 200, utils.SuccessResponse(map[string]any{"conversations": cs, "limit": limit, "offset": offset}), nil
}
func (s *ConversationServiceImpl) Get(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	uid, e := cid(r)
	if e != nil {
		return 401, nil, e
	}
	conversationID := chi.URLParam(r, "conversationID")
	if uuid.Validate(conversationID) != nil {
		return 400, nil, errs.ErrValidation
	}
	c, e := s.repo.Get(r.Context(), conversationID, uid)
	if e != nil {
		return 404, nil, e
	}
	return 200, utils.SuccessResponse(c), nil
}
