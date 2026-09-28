package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type GroupConversationService interface {
	CreateGroup(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	GetGroup(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	UpdateGroup(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	ListGroupMembers(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	AddGroupMembers(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	RemoveGroupMember(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	LeaveGroup(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	UpdateGroupMemberRole(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	CreateGroupInvite(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	ListGroupInvites(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	RevokeGroupInvite(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
	AcceptGroupInvite(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error)
}

func (s *GroupConversationServiceImpl) CreateGroupInvite(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, err := groupIDs(r, false)
	if err != nil {
		return groupInputError(err)
	}
	var q struct {
		ExpiresInMinutes int  `json:"expires_in_minutes"`
		MaxUses          *int `json:"max_uses"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&q) != nil || q.ExpiresInMinutes < 1 || q.ExpiresInMinutes > 43200 || (q.MaxUses != nil && (*q.MaxUses < 1 || *q.MaxUses > 10000)) {
		return 400, nil, errs.ErrValidation
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return 500, nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	invite, err := s.repo.CreateGroupInvite(r.Context(), id, actor, repository.HashGroupInviteToken(token), time.Now().UTC().Add(time.Duration(q.ExpiresInMinutes)*time.Minute), q.MaxUses)
	if err != nil {
		return groupError(err)
	}
	return 201, utils.SuccessResponse(map[string]any{"invite": invite, "token": token}), nil
}

func (s *GroupConversationServiceImpl) ListGroupInvites(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, err := groupIDs(r, false)
	if err != nil {
		return groupInputError(err)
	}
	invites, err := s.repo.ListGroupInvites(r.Context(), id, actor)
	if err != nil {
		return groupError(err)
	}
	return 200, utils.SuccessResponse(map[string]any{"invites": invites}), nil
}

func (s *GroupConversationServiceImpl) RevokeGroupInvite(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, err := groupIDs(r, false)
	if err != nil {
		return groupInputError(err)
	}
	inviteID := chi.URLParam(r, "inviteID")
	if uuid.Validate(inviteID) != nil {
		return 400, nil, errs.ErrValidation
	}
	if err = s.repo.RevokeGroupInvite(r.Context(), id, actor, inviteID); err != nil {
		return groupError(err)
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "revoked"}), nil
}

func (s *GroupConversationServiceImpl) AcceptGroupInvite(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, err := cid(r)
	if err != nil {
		return groupInputError(err)
	}
	token := chi.URLParam(r, "token")
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(token)
	if decodeErr != nil || len(decoded) != 32 {
		return 400, nil, errs.ErrValidation
	}
	conversationID, err := s.repo.AcceptGroupInvite(r.Context(), actor, repository.HashGroupInviteToken(token))
	if err != nil {
		return groupError(err)
	}
	return 200, utils.SuccessResponse(map[string]string{"conversation_id": conversationID}), nil
}

type GroupConversationServiceImpl struct {
	repo          repository.GroupConversationRepository
	conversations repository.ConversationRepository
}

func NewGroupConversationService(repo repository.GroupConversationRepository, conversations repository.ConversationRepository) *GroupConversationServiceImpl {
	return &GroupConversationServiceImpl{repo: repo, conversations: conversations}
}

func groupError(err error) (int, *utils.APIResponse, error) {
	switch {
	case errors.Is(err, errs.ErrUnauthorized):
		return http.StatusUnauthorized, nil, err
	case errors.Is(err, errs.ErrNotFound):
		return 404, nil, err
	case errors.Is(err, errs.ErrForbidden):
		return 403, nil, err
	case errors.Is(err, errs.ErrInactiveUser):
		return 403, nil, err
	case errors.Is(err, errs.ErrConflict):
		return 409, nil, err
	default:
		return 500, nil, err
	}
}

func groupInputError(err error) (int, *utils.APIResponse, error) {
	if errors.Is(err, errs.ErrUnauthorized) {
		return groupError(err)
	}
	return http.StatusBadRequest, nil, err
}
func groupIDs(r *http.Request, includeUser bool) (string, string, string, error) {
	actor, err := cid(r)
	if err != nil {
		return "", "", "", err
	}
	conversationID := chi.URLParam(r, "conversationID")
	if uuid.Validate(conversationID) != nil {
		return "", "", "", errs.ErrValidation
	}
	userID := ""
	if includeUser {
		userID = chi.URLParam(r, "userID")
		if uuid.Validate(userID) != nil {
			return "", "", "", errs.ErrValidation
		}
	}
	return actor, conversationID, userID, nil
}
func validMemberIDs(actor string, ids []string) bool {
	if len(ids) > 100 {
		return false
	}
	seen := map[string]bool{actor: true}
	for _, id := range ids {
		if uuid.Validate(id) != nil || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (s *GroupConversationServiceImpl) CreateGroup(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, e := cid(r)
	if e != nil {
		return 401, nil, e
	}
	if uuid.Validate(actor) != nil {
		return 400, nil, errs.ErrValidation
	}
	var q struct {
		Name        string   `json:"name"`
		Description *string  `json:"description"`
		MemberIDs   []string `json:"member_ids"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&q) != nil {
		return 400, nil, errs.ErrValidation
	}
	q.Name = strings.TrimSpace(q.Name)
	if len(q.Name) < 1 || len(q.Name) > 100 || !validMemberIDs(actor, q.MemberIDs) {
		return 400, nil, errs.ErrValidation
	}
	if q.Description != nil {
		v := strings.TrimSpace(*q.Description)
		q.Description = &v
		if len(v) > 2000 {
			return 400, nil, errs.ErrValidation
		}
	}
	c, e := s.repo.CreateGroup(r.Context(), actor, q.Name, q.Description, q.MemberIDs)
	if e != nil {
		return groupError(e)
	}
	return 201, utils.SuccessResponse(c), nil
}
func (s *GroupConversationServiceImpl) GetGroup(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, e := groupIDs(r, false)
	if e != nil {
		return groupInputError(e)
	}
	c, e := s.conversations.Get(r.Context(), id, actor)
	if e != nil {
		return groupError(e)
	}
	if c.Kind != "group" {
		return 404, nil, errs.ErrNotFound
	}
	return 200, utils.SuccessResponse(c), nil
}
func (s *GroupConversationServiceImpl) UpdateGroup(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, e := groupIDs(r, false)
	if e != nil {
		return groupInputError(e)
	}
	var q struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || (q.Name == nil && q.Description == nil) {
		return 400, nil, errs.ErrValidation
	}
	if q.Name != nil {
		v := strings.TrimSpace(*q.Name)
		q.Name = &v
		if len(v) < 1 || len(v) > 100 {
			return 400, nil, errs.ErrValidation
		}
	}
	if q.Description != nil {
		v := strings.TrimSpace(*q.Description)
		q.Description = &v
		if len(v) > 2000 {
			return 400, nil, errs.ErrValidation
		}
	}
	c, e := s.repo.UpdateGroup(r.Context(), id, actor, q.Name, q.Description)
	if e != nil {
		return groupError(e)
	}
	return 200, utils.SuccessResponse(c), nil
}
func (s *GroupConversationServiceImpl) ListGroupMembers(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, e := groupIDs(r, false)
	if e != nil {
		return groupInputError(e)
	}
	members, e := s.repo.ListGroupMembers(r.Context(), id, actor)
	if e != nil {
		return groupError(e)
	}
	return 200, utils.SuccessResponse(map[string]any{"members": members}), nil
}
func (s *GroupConversationServiceImpl) AddGroupMembers(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, e := groupIDs(r, false)
	if e != nil {
		return groupInputError(e)
	}
	var q struct {
		MemberIDs []string `json:"member_ids"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&q) != nil || len(q.MemberIDs) == 0 || !validMemberIDs(actor, q.MemberIDs) {
		return 400, nil, errs.ErrValidation
	}
	members, e := s.repo.AddGroupMembers(r.Context(), id, actor, q.MemberIDs)
	if e != nil {
		return groupError(e)
	}
	return 200, utils.SuccessResponse(map[string]any{"members": members}), nil
}
func (s *GroupConversationServiceImpl) RemoveGroupMember(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, target, e := groupIDs(r, true)
	if e != nil {
		return groupInputError(e)
	}
	e = s.repo.RemoveGroupMember(r.Context(), id, actor, target)
	if e != nil {
		return groupError(e)
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "removed"}), nil
}
func (s *GroupConversationServiceImpl) LeaveGroup(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, _, e := groupIDs(r, false)
	if e != nil {
		return groupInputError(e)
	}
	e = s.repo.LeaveGroup(r.Context(), id, actor)
	if e != nil {
		return groupError(e)
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "left"}), nil
}
func (s *GroupConversationServiceImpl) UpdateGroupMemberRole(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	actor, id, target, e := groupIDs(r, true)
	if e != nil {
		return groupInputError(e)
	}
	var q struct {
		Role string `json:"role"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || (q.Role != model.GroupRoleOwner && q.Role != model.GroupRoleAdmin && q.Role != model.GroupRoleMember) {
		return 400, nil, errs.ErrValidation
	}
	e = s.repo.UpdateGroupMemberRole(r.Context(), id, actor, target, q.Role)
	if e != nil {
		return groupError(e)
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "updated", "role": q.Role}), nil
}
