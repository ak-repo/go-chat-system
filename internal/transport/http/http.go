package http

import (
	"net/http"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/ak-repo/go-chat-system/internal/domain"
	"github.com/ak-repo/go-chat-system/internal/repository/postgres"
	"github.com/ak-repo/go-chat-system/internal/service"
	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"github.com/ak-repo/go-chat-system/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	userService domain.UserService
	jwt         *jwt.JWTManager
}

func NewRoutes(db *pgxpool.Pool, cfg *config.Config) http.Handler {

	mux := http.NewServeMux()
	//Repositories
	userRepo := postgres.NewUserRepo(db)

	// Services
	userService := service.NewUserService(userRepo)

	//JWT manager
	manager := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Expiry.Abs())

	httpHandler := &Handler{userService: userService, jwt: manager}

	// auth & user
	mux.Handle("/register", Chain(http.HandlerFunc(httpHandler.Register), JSON, AllowMethods("POST")))
	mux.Handle("/login", Chain(http.HandlerFunc(httpHandler.Login), JSON, AllowMethods("POST")))
	mux.Handle("/home", Chain(http.HandlerFunc(httpHandler.Home), JSON, AllowMethods("GET"), JWT(manager)))

	return Chain(
		mux,
		Recover,
		Logger(logger.Log),
		CORS,
	)
}
