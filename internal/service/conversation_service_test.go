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
}

func (f *fakeConversationRepo) CreateOrGetDirect(context.Context, string, string) (*model.Conversation, error) {
	return f.conversation, nil
}

func (f *fakeConversationRepo) Get(_ context.Context, id, _ string) (*model.Conversation, error) {
	f.gotID = id
	return f.conversation, nil
}

func (f *fakeConversationRepo) List(context.Context, string, int, int) ([]*model.Conversation, error) {
	return nil, nil
}

func (f *fakeConversationRepo) IsMember(context.Context, string, string) (bool, error) {
	return true, nil
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
