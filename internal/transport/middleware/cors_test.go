package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/platform/config"
)

func TestCORSAllowsImageCaptionPatch(t *testing.T) {
	previous := config.Config.CORS
	defer func() { config.Config.CORS = previous }()
	config.Config.CORS.AllowedOrigins = []string{"http://localhost:5173"}
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/messages/image", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	recorder := httptest.NewRecorder()
	CORS()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("preflight reached handler") })).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent || !strings.Contains(recorder.Header().Get("Access-Control-Allow-Methods"), "PATCH") {
		t.Fatalf("caption PATCH not allowed")
	}
}
