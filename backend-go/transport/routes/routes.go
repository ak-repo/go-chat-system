package routes

import (
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/transport/handler"
	"github.com/ak-repo/go-chat-system/transport/injector"
	"github.com/ak-repo/go-chat-system/transport/websocket"
	"github.com/ak-repo/go-chat-system/transport/wrapper"
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

	// injector -> contains all services and repository
	app := injector.Init()

	// Health checks
	r.Get("/redis-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis down :"+err.Error(), http.StatusServiceUnavailable)
			return
		}

		w.Write([]byte("ok"))

	})
	r.Get("/db-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.GetDB().Ping(r.Context()); err != nil {
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
		r.Get("/users", wrapper.HTTPResponseWrapper(app.UserService.SearchUser))
	})

	// Frineds routes
	v1.Route("/friends", func(r chi.Router) {
		r.Use(mdware.AuthMiddleware())
		r.Use(mdware.RateLimitRedis(mdware.UserKey, 10, time.Minute))

		r.Get("/", wrapper.HTTPResponseWrapper(app.FriendService.ListFriends))
		r.Post("/sent-req", wrapper.HTTPResponseWrapper(app.FriendService.CreateRequest))
		r.Post("/block", wrapper.HTTPResponseWrapper(app.FriendService.BlockUser))
		r.Post("/accept", wrapper.HTTPResponseWrapper(app.FriendService.AcceptRequest))

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

// friendHandler := handler.NewFriendHandler(friendService)

// r.Post("/friends/request", friendHandler.SendRequest)
// r.Post("/friends/accept", friendHandler.AcceptRequest)
// r.Post("/friends/block", friendHandler.BlockUser)
// r.Get("/friends", friendHandler.ListFriends)
