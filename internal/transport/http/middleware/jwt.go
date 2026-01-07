package middleware

import (
	"context"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/jwt"
)

/*
   =========================
   JWT Middleware
   =========================
*/

func JWT(manager *jwt.JWTManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("access")
			if err != nil {
				if err == http.ErrNoCookie {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			claims, err := manager.ValidateToken(cookie.Value)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "userID", claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
