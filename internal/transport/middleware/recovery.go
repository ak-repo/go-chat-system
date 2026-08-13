package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/ak-repo/go-chat-system/internal/shared/logger"
	"go.uber.org/zap"
)

/*
   =========================
   Panic Recovery
   =========================
*/

func Recover() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := GetRequestID(r.Context())
					if requestID == "" {
						requestID = "unknown"
					}

					fields := []zap.Field{
						zap.String("request_id", requestID),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("remote_addr", r.RemoteAddr),
						zap.String("user_agent", r.UserAgent()),
						zap.String("panic", fmt.Sprint(err)),
						zap.ByteString("stack", debug.Stack()),
					}
					if userID, ok := r.Context().Value(UserIDKey).(string); ok && userID != "" {
						fields = append(fields, zap.String("user_id", userID))
					}

					logger.L().Error("panic recovered", fields...)

					if sw, ok := w.(*statusResponseWriter); ok && sw.wrote {
						return
					}
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
