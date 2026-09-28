package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type fakeGroupRepo struct {
	actor, conversationID, target, role, name string
	members                                   []string
	err                                       error
}

func (f *fakeGroupRepo) CreateGroup(_ context.Context, actor, name string, _ *string, members []string) (*model.Conversation, error) {
	f.actor, f.name, f.members = actor, name, members
	return &model.Conversation{ID: uuid.NewString(), Kind: "group", Name: name}, f.err
}
func (f *fakeGroupRepo) UpdateGroup(_ context.Context, cid, actor string, _ *string, _ *string) (*model.Conversation, error) {
	f.actor, f.conversationID = actor, cid
	return &model.Conversation{ID: cid, Kind: "group"}, f.err
}
func (f *fakeGroupRepo) ListGroupMembers(_ context.Context, cid, actor string) ([]*model.GroupMember, error) {
	f.actor, f.conversationID = actor, cid
	return []*model.GroupMember{}, f.err
}
func (f *fakeGroupRepo) AddGroupMembers(_ context.Context, cid, actor string, members []string) ([]*model.GroupMember, error) {
	f.actor, f.conversationID, f.members = actor, cid, members
	return []*model.GroupMember{}, f.err
}
func (f *fakeGroupRepo) RemoveGroupMember(_ context.Context, cid, actor, target string) error {
	f.actor, f.conversationID, f.target = actor, cid, target
	return f.err
}
func (f *fakeGroupRepo) LeaveGroup(_ context.Context, cid, actor string) error {
	f.actor, f.conversationID = actor, cid
	return f.err
}
func (f *fakeGroupRepo) UpdateGroupMemberRole(_ context.Context, cid, actor, target, role string) error {
	f.actor, f.conversationID, f.target, f.role = actor, cid, target, role
	return f.err
}
func (f *fakeGroupRepo) CreateGroupInvite(_ context.Context, cid, actor string, _ []byte, expires time.Time, max *int) (*model.GroupInvite, error) {
	f.actor, f.conversationID = actor, cid
	return &model.GroupInvite{ID: uuid.NewString(), ConversationID: cid, CreatorID: actor, ExpiresAt: expires, MaxUses: max}, f.err
}
func (f *fakeGroupRepo) ListGroupInvites(_ context.Context, cid, actor string) ([]*model.GroupInvite, error) {
	f.actor, f.conversationID = actor, cid
	return []*model.GroupInvite{}, f.err
}
func (f *fakeGroupRepo) RevokeGroupInvite(_ context.Context, cid, actor, invite string) error {
	f.actor, f.conversationID, f.target = actor, cid, invite
	return f.err
}
func (f *fakeGroupRepo) AcceptGroupInvite(_ context.Context, actor string, _ []byte) (string, error) {
	f.actor = actor
	return f.conversationID, f.err
}

func groupRequest(method, body, actor, cidValue, target string) *http.Request {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	ctx := req.Context()
	if actor != "" {
		ctx = context.WithValue(ctx, middleware.UserIDKey, actor)
	}
	rc := chi.NewRouteContext()
	rc.URLParams.Add("conversationID", cidValue)
	if target != "" {
		rc.URLParams.Add("userID", target)
	}
	return req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rc))
}

func TestCreateGroupInviteReturnsTokenOnce(t *testing.T) {
	actor, conversationID := uuid.NewString(), uuid.NewString()
	repo := &fakeGroupRepo{}
	svc := NewGroupConversationService(repo, &fakeConversationRepo{})
	status, response, err := svc.CreateGroupInvite(httptest.NewRecorder(), groupRequest(http.MethodPost, `{"expires_in_minutes":60,"max_uses":2}`, actor, conversationID, ""))
	if err != nil || status != http.StatusCreated || repo.actor != actor {
		t.Fatalf("create invite: %d %v %#v", status, err, repo)
	}
	data, ok := response.Data.(map[string]any)
	if !ok || len(data["token"].(string)) < 40 {
		t.Fatalf("missing opaque token: %#v", response.Data)
	}
}

func TestCreateGroupUsesAuthenticatedActorAndValidatesMembers(t *testing.T) {
	actor, member := uuid.NewString(), uuid.NewString()
	repo := &fakeGroupRepo{}
	svc := NewGroupConversationService(repo, &fakeConversationRepo{})
	req := groupRequest(http.MethodPost, `{"name":"  Project  ","member_ids":["`+member+`"]}`, actor, "", "")
	status, _, err := svc.CreateGroup(httptest.NewRecorder(), req)
	if err != nil || status != http.StatusCreated || repo.actor != actor || repo.name != "Project" || len(repo.members) != 1 {
		t.Fatalf("unexpected create result: status=%d err=%v repo=%#v", status, err, repo)
	}
	req = groupRequest(http.MethodPost, `{"name":"Project","member_ids":["`+actor+`"]}`, actor, "", "")
	status, _, err = svc.CreateGroup(httptest.NewRecorder(), req)
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected duplicate actor validation, got %d %v", status, err)
	}
}

func TestGroupRoleUpdateMapsAuthorizationAndConflict(t *testing.T) {
	actor, target, conversationID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	repo := &fakeGroupRepo{err: errs.ErrForbidden}
	svc := NewGroupConversationService(repo, &fakeConversationRepo{})
	req := groupRequest(http.MethodPatch, `{"role":"admin"}`, actor, conversationID, target)
	status, _, err := svc.UpdateGroupMemberRole(httptest.NewRecorder(), req)
	if status != http.StatusForbidden || !errors.Is(err, errs.ErrForbidden) || repo.actor != actor {
		t.Fatalf("expected server actor and forbidden, got %d %v %#v", status, err, repo)
	}
	repo.err = errs.ErrConflict
	req = groupRequest(http.MethodPatch, `{"role":"admin"}`, actor, conversationID, target)
	status, _, err = svc.UpdateGroupMemberRole(httptest.NewRecorder(), req)
	if status != http.StatusConflict || !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("expected final-owner conflict, got %d %v", status, err)
	}
}

func TestGroupRouteRequiresAuthenticatedContext(t *testing.T) {
	svc := NewGroupConversationService(&fakeGroupRepo{}, &fakeConversationRepo{})
	status, _, err := svc.ListGroupMembers(httptest.NewRecorder(), groupRequest(http.MethodGet, "", "", uuid.NewString(), ""))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %d %v", status, err)
	}
}
