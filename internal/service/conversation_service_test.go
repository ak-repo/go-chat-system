package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type fakeConversationRepo struct {
	conversation *model.Conversation
	gotID        string
	err          error
	memberErr    error
	listedUser   string
	listedLimit  int
	listedOffset int
	createdUser  string
	createdOther string
}

func (f *fakeConversationRepo) CreateOrGetDirect(_ context.Context, a, b string) (*model.Conversation, error) {
	f.createdUser, f.createdOther = a, b
	return f.conversation, f.err
}

func (f *fakeConversationRepo) Get(_ context.Context, id, _ string) (*model.Conversation, error) {
	f.gotID = id
	return f.conversation, nil
}

func (f *fakeConversationRepo) List(_ context.Context, user string, limit, offset int) ([]*model.Conversation, error) {
	f.listedUser, f.listedLimit, f.listedOffset = user, limit, offset
	if f.conversation == nil {
		return nil, f.err
	}
	return []*model.Conversation{f.conversation}, f.err
}

func (f *fakeConversationRepo) IsMember(context.Context, string, string) (bool, error) {
	return true, f.memberErr
}

func requestWithRouteParam(id, userID string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("conversationID", id)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+id, nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeContext)
	return req.WithContext(ctx)
}

func TestConversationGetUsesChiRouteParameter(t *testing.T) {
	id := uuid.NewString()
	repo := &fakeConversationRepo{conversation: &model.Conversation{ID: id, Kind: "direct", UserOneID: "user-1", UserTwoID: "user-2", CreatedAt: time.Now(), ModifiedAt: time.Now()}}
	service := NewConversationService(repo)

	status, _, err := service.Get(httptest.NewRecorder(), requestWithRouteParam(id, "user-1"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if repo.gotID != id {
		t.Fatalf("expected repository ID %q, got %q", id, repo.gotID)
	}
}

func TestConversationGetRejectsMissingRouteParameter(t *testing.T) {
	repo := &fakeConversationRepo{}
	service := NewConversationService(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations//", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))

	status, _, err := service.Get(httptest.NewRecorder(), req)
	if status != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, status)
	}
	if !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if repo.gotID != "" {
		t.Fatalf("repository should not be called, got ID %q", repo.gotID)
	}
}

func TestConversationCreateRequiresFriendshipAndRejectsBlocks(t *testing.T) {
	repo := &fakeConversationRepo{conversation: &model.Conversation{ID: uuid.NewString()}}
	req := authenticatedRequest(http.MethodPost, `{"user_id":"user-2"}`, "user-1")
	status, _, err := NewConversationService(repo, fakeFriendRepo{areFriends: false}, fakeBlockRepo{}).Create(httptest.NewRecorder(), req)
	if status != http.StatusForbidden || !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected friendship authorization, got %d, %v", status, err)
	}
	req = authenticatedRequest(http.MethodPost, `{"user_id":"user-2"}`, "user-1")
	status, _, err = NewConversationService(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{blocked: true}).Create(httptest.NewRecorder(), req)
	if status != http.StatusForbidden || !errors.Is(err, errs.ErrBlockedRelationship) {
		t.Fatalf("expected block authorization, got %d, %v", status, err)
	}
}

func TestConversationCreatePropagatesDependencyError(t *testing.T) {
	repo := &fakeConversationRepo{conversation: &model.Conversation{ID: uuid.NewString()}}
	req := authenticatedRequest(http.MethodPost, `{"user_id":"user-2"}`, "user-1")
	status, _, err := NewConversationService(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{err: errors.New("database unavailable")}).Create(httptest.NewRecorder(), req)
	if status != http.StatusInternalServerError || !errors.Is(err, errs.ErrInternal) {
		t.Fatalf("expected internal dependency error, got %d, %v", status, err)
	}
}

func TestConversationListUsesAuthenticatedIdentityAndPagination(t *testing.T) {
	repo := &fakeConversationRepo{conversation: &model.Conversation{ID: uuid.NewString()}}
	service := NewConversationService(repo)
	status, _, err := service.List(httptest.NewRecorder(), authenticatedRequest(http.MethodGet, "", "user-1"))
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected conversation list, got %d %v", status, err)
	}
	req := authenticatedRequest(http.MethodGet, "", "user-1")
	req.URL.RawQuery = "limit=7&offset=3"
	status, _, err = service.List(httptest.NewRecorder(), req)
	if status != http.StatusOK || err != nil || repo.listedUser != "user-1" || repo.listedLimit != 7 || repo.listedOffset != 3 {
		t.Fatalf("unexpected list call: %d %v %#v", status, err, repo)
	}
	status, _, err = service.List(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected list auth error, got %d %v", status, err)
	}
}

func TestConversationCreatePersistsTrimmedTarget(t *testing.T) {
	repo := &fakeConversationRepo{conversation: &model.Conversation{ID: uuid.NewString()}}
	status, _, err := NewConversationService(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{}).Create(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"user_id":" user-2 "}`, "user-1"))
	if status != http.StatusOK || err != nil || repo.createdUser != "user-1" || repo.createdOther != "user-2" {
		t.Fatalf("expected conversation creation, got %d %v %#v", status, err, repo)
	}
}
