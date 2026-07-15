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
	"github.com/google/uuid"
)

type fakeMessageRepo struct {
	messages model.Messages
	err      error
}

func (f fakeMessageRepo) CreateMessage(context.Context, *model.Message) error {
	return nil
}

func (f fakeMessageRepo) GetMessagesByReceiver(context.Context, string, int, int) (model.Messages, error) {
	return nil, nil
}

func (f fakeMessageRepo) GetMessagesBetweenUsers(context.Context, string, string, int, int) (model.Messages, error) {
	return f.messages, f.err
}

func TestGetMessagesUsesMiddlewareUserIDKey(t *testing.T) {
	repo := fakeMessageRepo{
		messages: model.Messages{
			{
				ID:         uuid.NewString(),
				SenderID:   "user-1",
				ReceiverID: "user-2",
				Body:       "hello",
				CreatedAt:  time.Now().UTC(),
				ModifiedAt: time.Now().UTC(),
			},
		},
	}

	service := NewMessageServiceImpl(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-2&limit=50&offset=0", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))

	status, resp, err := service.GetMessages(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp == nil || resp.Data == nil {
		t.Fatalf("expected response data, got %#v", resp)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %T", resp.Data)
	}
	if got := data["limit"]; got != 50 {
		t.Fatalf("expected limit 50, got %#v", got)
	}
	if got := data["offset"]; got != 0 {
		t.Fatalf("expected offset 0, got %#v", got)
	}
}

func TestGetMessagesRejectsMissingMiddlewareUserIDKey(t *testing.T) {
	repo := fakeMessageRepo{}
	service := NewMessageServiceImpl(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-2", nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", "user-1"))

	status, resp, err := service.GetMessages(httptest.NewRecorder(), req)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, status)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %#v", resp)
	}
	if err == nil || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestGetMessagesReturnsEmptySliceWhenNoMessages(t *testing.T) {
	repo := fakeMessageRepo{messages: nil}
	service := NewMessageServiceImpl(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-2", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))

	status, resp, err := service.GetMessages(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %T", resp.Data)
	}
	msgs, ok := data["messages"].(model.Messages)
	if !ok {
		t.Fatalf("expected model.Messages, got %T", data["messages"])
	}
	if msgs == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(msgs))
	}
}
