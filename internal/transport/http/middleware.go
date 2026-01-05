package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"go.uber.org/zap"
)

type Middleware func(http.Handler) http.Handler

type ctxKey string

const userIDKey ctxKey = "userID"
const roleKey ctxKey = "role"

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	// Default status is 200 if WriteHeader not called
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func Logger(log *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			sw := &statusResponseWriter{
				ResponseWriter: w,
				status:         http.StatusOK, // default
			}

			next.ServeHTTP(sw, r)

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", sw.status),
				zap.Int("bytes", sw.bytes),
				zap.Duration("latency", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			}

			switch {
			case sw.status >= 500:
				log.Error("http request failed", fields...)
			case sw.status >= 400:
				log.Warn("http request client error", fields...)
			default:
				log.Info("http request", fields...)
			}

		})
	}
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func JSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
func BodyLimit(max int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

func AllowMethods(methods ...string) Middleware {
	allowed := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		allowed[m] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := allowed[r.Method]; !ok {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func JWT(manager *jwt.JWTManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := r.Cookie("access")
			if err != nil || token.Value == "" {
				if err == http.ErrNoCookie {
					http.Error(w, "cookie not found", http.StatusUnauthorized)
					return
				}
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}

			// Validate token
			claims, err := manager.ValidateToken(token.Value)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.ID)
			next.ServeHTTP(w, r.WithContext(ctx))

		})
	}
}
