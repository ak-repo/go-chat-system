package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/ak-repo/go-chat-system/pkg/jwt"
)

type ContextKey string

const UserIDKey ContextKey = "userID"

func AuthMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			// 1. Check Authorization header first
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}

			// 2. Fallback to cookie if no bearer token
			if token == "" {
				cookie, err := r.Cookie("access")
				if err != nil {
					if err == http.ErrNoCookie {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				token = cookie.Value
			}

			// 3. Validate token
			claims, err := jwt.ValidateToken(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// 4. Inject user ID into request context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
