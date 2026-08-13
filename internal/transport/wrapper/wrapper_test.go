package wrapper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/logger"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	mdware "github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestHTTPResponseWrapperLogsInternalErrorSafely(t *testing.T) {
	observed, logs := observer.New(zap.ErrorLevel)
	prev := logger.Logger
	logger.Logger = zap.New(observed)
	t.Cleanup(func() { logger.Logger = prev })

	internalErr := errs.Wrap("service.UserService.Register", errs.Wrap("repository.UserRepository.CreateUser", errors.New("duplicate key value violates unique constraint users_email_key")))
	handler := HTTPResponseWrapper(func(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
		return http.StatusInternalServerError, nil, internalErr
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register?token=secret", nil)
	ctx := context.WithValue(req.Context(), mdware.RequestIDKey, "req-123")
	ctx = context.WithValue(ctx, mdware.UserIDKey, "user-1")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "duplicate key") {
		t.Fatalf("response leaked internal error: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "an error occurred") {
		t.Fatalf("response body = %s, want safe message", rec.Body.String())
	}

	entries := logs.FilterMessage("http handler error").All()
	if len(entries) != 1 {
		t.Fatalf("logged entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != "req-123" || fields["path"] != "/api/v1/auth/register" || fields["user_id"] != "user-1" {
		t.Fatalf("unexpected log fields: %#v", fields)
	}
	if _, ok := fields["trace"]; !ok {
		t.Fatalf("log fields missing trace: %#v", fields)
	}
}
