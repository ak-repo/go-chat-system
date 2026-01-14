package routes

import (
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/transport/handler"
	"github.com/ak-repo/go-chat-system/transport/websocket"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	mdware "github.com/ak-repo/go-chat-system/transport/middleware"
)

func Router() chi.Router {

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(mdware.CORS())

	v1 := chi.NewRouter()
	api := chi.NewRouter()

	// Health checks
	r.Get("/redis-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis down :"+err.Error(), http.StatusServiceUnavailable)
			return
		}

		w.Write([]byte("ok"))

	})
	r.Get("/db-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.DB.Ping(r.Context()); err != nil {
			http.Error(w, "db down :"+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	//----------------------------------------------------------------

	// Authentication
	v1.Group(func(r chi.Router) {
		r.Use(mdware.RateLimitRedis(mdware.IPKey, 10, time.Minute))
		r.Post("/register", handler.Register)
		r.Post("/login", handler.UserLogin)
	})

	// User private routes
	v1.Route("/user", func(r chi.Router) {
		r.Use(mdware.AuthMiddleware())
		r.Use(mdware.RateLimitRedis(mdware.UserKey, 10, time.Minute))
		r.Get("/home", handler.Home)
	})

	//----------------Websocket----------------------
	hub := websocket.NewHub()
	go hub.Run()

	wsHandler := handler.NewWebsocketHandler(hub)
	v1.Group(func(r chi.Router) {
		r.Use(mdware.AuthMiddleware())
		r.Get("/ws", wsHandler.Handler)
	})

	api.Mount("/v1", v1)
	r.Mount("/api", api)

	return r
}
