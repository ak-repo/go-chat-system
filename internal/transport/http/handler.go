package http

import (
	"net/http"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/ak-repo/go-chat-system/internal/domain"
	mw "github.com/ak-repo/go-chat-system/internal/transport/http/middleware"
	ws_pkg "github.com/ak-repo/go-chat-system/internal/transport/websocket"
	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"github.com/ak-repo/go-chat-system/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	userService domain.UserService
	jwt         *jwt.JWTManager
	hub         *ws_pkg.Hub
}

func NewHandler(db *pgxpool.Pool, cfg *config.Config, hub *ws_pkg.Hub, redis *redis.Client) http.Handler {
	mux := http.NewServeMux()
	routes(mux, db, cfg, hub, redis)

	return mw.Chain(
		mux,
		mw.Recover(),
		mw.Logger(logger.Log),
		mw.CORS(),
	)
}
