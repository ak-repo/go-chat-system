package middleware /*
   =========================
   CORS Middleware (from config; tighten allow_origins for production)
   =========================
*/

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ak-repo/go-chat-system/config"
)

func CORS() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origins := config.Config.CORS.AllowOrigins
			if len(origins) == 0 {
				origins = []string{fmt.Sprintf("%s:%d", config.Config.CORS.Host, config.Config.CORS.Port)}
			}
			origin := r.Header.Get("Origin")
			allowed := ""
			if origin != "" {
				for _, o := range origins {
					if strings.TrimSpace(o) == origin {
						allowed = origin
						break
					}
				}
			}
			if allowed == "" && len(origins) == 1 {
				allowed = origins[0]
			}
			if allowed != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowed)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
