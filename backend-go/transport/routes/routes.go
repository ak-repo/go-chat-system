package routes

import (
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/transport/injector"
	mdware "github.com/ak-repo/go-chat-system/transport/middleware"
	"github.com/ak-repo/go-chat-system/transport/wrapper"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func Router() chi.Router {
	r := chi.NewRouter()

	// global middleware: request ID first so it's available for logging and errors
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(mdware.Logger())
	r.Use(mdware.Recover())
	r.Use(mdware.CORS())

	// injector -> contains all services and repository
	app := injector.Init()

	// ---------------- API v1 ----------------
	r.Route("/api/v1", func(v1 chi.Router) {

		// ---------------- Auth ----------------
		v1.Group(func(auth chi.Router) {
			auth.Use(mdware.RateLimitRedis(mdware.IPKey, 10, time.Minute))
			auth.Post("/auth/register", wrapper.HTTPResponseWrapper(app.UserService.Register))
			auth.Post("/auth/login", wrapper.HTTPResponseWrapper(app.UserService.Login))
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

			// Chats and messages
			pr.Route("/chats", func(cr chi.Router) {
				cr.Get("/", wrapper.HTTPResponseWrapper(app.ChatService.ListMyChats))
				cr.Post("/", wrapper.HTTPResponseWrapper(app.ChatService.GetOrCreateDM))
				cr.Get("/{chatID}/messages", wrapper.HTTPResponseWrapper(app.MessageService.ListMessages))
			})

			// Websocket
			wsHandler := wrapper.NewWebsocketHandler(app.Hub)
			pr.Get("/ws", wsHandler.Handler)
		})
	})

	// ---------------- Health checks (for orchestrator: use /live and /ready) ----------------
	// Liveness: process is running (no dependencies)
	r.Get("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Readiness: app can serve traffic (DB + Redis up)
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := database.GetDB().Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("db not ready"))
			return
		}
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("redis not ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/redis-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("redis down"))
			return
		}
		w.Write([]byte("ok"))
	})

	r.Get("/db-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.GetDB().Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("db down"))
			return
		}
		w.Write([]byte("ok"))
	})

	return r
}
