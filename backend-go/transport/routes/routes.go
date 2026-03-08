package routes

import (
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/transport/injector"
	mdware "github.com/ak-repo/go-chat-system/transport/middleware"
	"github.com/ak-repo/go-chat-system/transport/websocket"
	"github.com/ak-repo/go-chat-system/transport/wrapper"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func Router() chi.Router {
	r := chi.NewRouter()

	// global middleware
	r.Use(middleware.Logger)
	r.Use(middleware.RequestID)
	r.Use(mdware.CORS())
	r.Use(mdware.Recover())

	// injector -> contains all services and repository
	app := injector.Init()

	// ---------------- API v1 ----------------
	r.Route("/api/v1", func(v1 chi.Router) {

		// ---------------- Auth ----------------
		v1.Group(func(auth chi.Router) {
			auth.Use(mdware.RateLimitRedis(mdware.IPKey, 10, time.Minute))
			auth.Post("/auth/register", wrapper.HTTPResponseWrapper(app.UserService.Register))
			auth.Post("/auth/login", wrapper.HTTPResponseWrapper(app.UserService.Login))
			auth.Post("/auth/refresh", wrapper.HTTPResponseWrapper(app.UserService.RefreshToken))
		})

		// ---------------- Protected routes ----------------
		v1.Group(func(pr chi.Router) {
			pr.Use(mdware.AuthMiddleware())
			pr.Use(mdware.RateLimitRedis(mdware.UserKey, 10, time.Minute))

			// Users
			pr.Get("/users", wrapper.HTTPResponseWrapper(app.UserService.SearchUser))

			// Friends
			pr.Get("/friends", wrapper.HTTPResponseWrapper(app.FriendService.ListFriends))

			// Friend Requests
			pr.Route("/friend-requests", func(fr chi.Router) {
				fr.Get("/", wrapper.HTTPResponseWrapper(app.FriendRequestService.GetAllRequests))
				fr.Post("/", wrapper.HTTPResponseWrapper(app.FriendRequestService.CreateRequest))
				fr.Post("/accept", wrapper.HTTPResponseWrapper(app.FriendRequestService.AcceptRequest))
				fr.Post("/cancel", wrapper.HTTPResponseWrapper(app.FriendRequestService.CancelRequest))
				fr.Post("/reject", wrapper.HTTPResponseWrapper(app.FriendRequestService.RejectRequest))
			})

			// Blocks
			pr.Route("/blocks", func(b chi.Router) {
				b.Post("/", wrapper.HTTPResponseWrapper(app.BlockService.BlockUser))
				b.Post("/unblock", wrapper.HTTPResponseWrapper(app.BlockService.UnblockUser))
			})

			// Messages
			pr.Get("/messages", wrapper.HTTPResponseWrapper(app.MessageService.GetMessages))

			// Websocket
			hub := websocket.NewHub(app.MessageService)
			go hub.Run()

			wsHandler := wrapper.NewWebsocketHandler(hub)
			pr.Get("/ws", wsHandler.Handler)
		})
	})

	// ---------------- Health checks ----------------
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis not ready", http.StatusServiceUnavailable)
			return
		}
		if err := database.GetDB().Ping(r.Context()); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	r.Get("/redis-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "redis down: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	r.Get("/db-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.GetDB().Ping(r.Context()); err != nil {
			http.Error(w, "db down: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	return r
}
