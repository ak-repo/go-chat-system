package middleware

import (
	"net"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/internal/platform/database"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func RateLimitRedis(
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
				database.RedisClient,
				[]string{key},
				time.Now().Unix(),
				int(window.Seconds()),
				limit,
				uuid.NewString(),
			).Int()

			if err != nil {
				// Authentication and recovery routes must fail closed when Redis is unavailable.
				http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
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
	// X-Forwarded-For is attacker-controlled unless a trusted proxy has been
	// explicitly configured. Use the peer address by default.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if ip == "" {
		return ""
	}
	return "rate:ip:" + ip
}

// For private apis
func UserKey(r *http.Request) string {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		return ""
	}
	return "rate:user:" + userID
}
