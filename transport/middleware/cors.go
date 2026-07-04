package middleware /*
   =========================
   CORS Middleware
   =========================
*/

import (
	"fmt"
	"net/http"

	"github.com/ak-repo/go-chat-system/config"
)

func isOriginAllowed(origin string) bool {
	allowedOrigins := config.Config.CORS.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{fmt.Sprintf("%s:%d", config.Config.CORS.Host, config.Config.CORS.Port)}
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func CORS() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
