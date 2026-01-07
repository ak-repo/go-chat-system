package http

import (
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/ak-repo/go-chat-system/internal/repository/postgres"
	"github.com/ak-repo/go-chat-system/internal/service"
	mw "github.com/ak-repo/go-chat-system/internal/transport/http/middleware"
	ws_pkg "github.com/ak-repo/go-chat-system/internal/transport/websocket"

	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func routes(mux *http.ServeMux, db *pgxpool.Pool, cfg *config.Config, hub *ws_pkg.Hub, redis *redis.Client) {

	// Redis health check
	mux.HandleFunc("/redis-healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := redis.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis down :"+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/db-healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "db down :"+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	// Repositories
	userRepo := postgres.NewUserRepo(db)

	// Services
	userService := service.NewUserService(userRepo)

	// JWT manager
	manager := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Expiry.Abs())

	httpHandler := &Handler{
		userService: userService,
		jwt:         manager,
		hub:         hub,
	}

	// WebSocket route
	mux.Handle(
		"/ws",
		mw.JWT(manager)(
			http.HandlerFunc(httpHandler.wsHandler),
		),
	)

	// Auth & user routes
	mux.Handle("/register", mw.Chain(
		http.HandlerFunc(httpHandler.Register),
		mw.JSON(),
		mw.AllowMethods("POST"),
		mw.RateLimitRedis(redis, mw.IPKey, 10, time.Minute),
	))

	mux.Handle("/login", mw.Chain(
		http.HandlerFunc(httpHandler.Login),
		mw.JSON(),
		mw.AllowMethods("POST"),
		mw.RateLimitRedis(redis, mw.IPKey, 10, time.Minute),
	))

	mux.Handle("/home", mw.Chain(
		http.HandlerFunc(httpHandler.Home),
		mw.JSON(),
		mw.AllowMethods("GET"),
		mw.JWT(manager),
		mw.RateLimitRedis(redis, mw.UserKey, 10, time.Minute),
	))

}
