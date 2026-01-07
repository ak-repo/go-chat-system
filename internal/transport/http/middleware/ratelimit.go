package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ak-repo/go-chat-system/pkg/helper"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func RateLimitRedis(
	rdb *redis.Client,
	keyFn func(*http.Request) string,
	limit int,
	window time.Duration,
) func(http.Handler) http.Handler {

	script := redis.NewScript(`
		redis.call("ZADD", KEYS[1], ARGV[1], ARGV[4])
		redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, ARGV[1] - ARGV[2])
		local count = redis.call("ZCARD", KEYS[1])
		if count > tonumber(ARGV[3]) then
			return 0
		end
		redis.call("EXPIRE", KEYS[1], ARGV[2])
		return 1
	`)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			key := keyFn(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := script.Run(
				r.Context(),
				rdb,
				[]string{key},
				time.Now().Unix(),
				int(window.Seconds()),
				limit,
				uuid.NewString(),
			).Int()

			if err != nil {
				// Fail open (recommended)
				next.ServeHTTP(w, r)
				return
			}

			if allowed == 0 {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IP based key for public routes
func IPKey(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			ip = host
		}
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	return "rate:ip:" + ip
}

// For private apis
func UserKey(r *http.Request) string {
	userID, ok := helper.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		return ""
	}
	return "rate:user:" + userID
}
