package middleware

import (
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/pkg/logger"
	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"
)

/*
   =========================
   Logger Middleware (structured logging with request_id)
   =========================
*/

func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sw := &statusResponseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(sw, r)

			fields := []zap.Field{
				zap.String("request_id", middleware.GetReqID(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", sw.status),
				zap.Int("bytes", sw.bytes),
				zap.Duration("latency", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
			}
			logger.Logger.Info("http request", fields...)
		})
	}
}
