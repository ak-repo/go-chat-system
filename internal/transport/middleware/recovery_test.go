package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/shared/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecoverLogsPanicWithStackAndSafeResponse(t *testing.T) {
	observed, logs := observer.New(zap.ErrorLevel)
	prev := logger.Logger
	logger.Logger = zap.New(observed)
	t.Cleanup(func() { logger.Logger = prev })

	handler := Recover()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("database password should not go to client")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?token=secret", nil)
	ctx := context.WithValue(req.Context(), RequestIDKey, "req-456")
	ctx = context.WithValue(ctx, UserIDKey, "user-2")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "database password") || strings.Contains(rec.Body.String(), "goroutine") {
		t.Fatalf("response leaked panic details: %s", rec.Body.String())
	}

	entries := logs.FilterMessage("panic recovered").All()
	if len(entries) != 1 {
		t.Fatalf("logged entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["request_id"] != "req-456" || fields["path"] != "/api/v1/messages" || fields["user_id"] != "user-2" {
		t.Fatalf("unexpected log fields: %#v", fields)
	}
	if _, ok := fields["stack"]; !ok {
		t.Fatalf("log fields missing stack: %#v", fields)
	}
}
