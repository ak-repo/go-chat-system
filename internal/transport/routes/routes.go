package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/internal/platform/database"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/jwt"
	"github.com/ak-repo/go-chat-system/internal/transport/injector"
	mdware "github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/ak-repo/go-chat-system/internal/transport/websocket"
	"github.com/ak-repo/go-chat-system/internal/transport/wrapper"
	"github.com/go-chi/chi"
)

var GlobalHub *websocket.Hub

func Router() chi.Router {
	r := chi.NewRouter()

	// global middleware
	r.Use(mdware.RequestID())
	r.Use(mdware.CORS())
	r.Use(mdware.Logger())
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
			auth.Post("/auth/password-reset/request", wrapper.HTTPResponseWrapper(app.AccountTokenService.RequestReset))
			auth.Post("/auth/password-reset/confirm", wrapper.HTTPResponseWrapper(app.AccountTokenService.ResetPassword))
			auth.Post("/auth/verification/request", wrapper.HTTPResponseWrapper(app.AccountTokenService.RequestVerification))
			auth.Post("/auth/verification/confirm", wrapper.HTTPResponseWrapper(app.AccountTokenService.Verify))
		})

		// ---------------- Protected routes ----------------
		v1.Group(func(pr chi.Router) {
			pr.Use(mdware.AuthMiddlewareWithSession(func(ctx context.Context, claims *jwt.Claims) bool {
				u, err := app.UserRepo.GetByID(ctx, claims.UserID)
				if err != nil || u == nil {
					return false
				}
				sr, ok := app.UserRepo.(repository.SessionRepository)
				if !ok {
					return false
				}
				active, err := sr.IsSessionActive(ctx, claims.UserID, claims.SessionID)
				return err == nil && active
			}))
			pr.Use(mdware.RateLimitRedis(mdware.UserKey, 120, time.Minute))
			pr.Post("/auth/logout", wrapper.HTTPResponseWrapper(app.UserService.Logout))

			// Users
			pr.Get("/users", wrapper.HTTPResponseWrapper(app.UserService.SearchUser))
			pr.Get("/users/me", wrapper.HTTPResponseWrapper(app.UserService.GetMe))
			pr.Patch("/users/me", wrapper.HTTPResponseWrapper(app.UserService.UpdateMe))
			pr.Post("/users/me/change-password", wrapper.HTTPResponseWrapper(app.UserService.ChangePassword))
			pr.Delete("/users/me", wrapper.HTTPResponseWrapper(app.UserService.Deactivate))
			pr.Post("/conversations", wrapper.HTTPResponseWrapper(app.ConversationService.Create))
			pr.Get("/conversations", wrapper.HTTPResponseWrapper(app.ConversationService.List))
			pr.Get("/conversations/{conversationID}", wrapper.HTTPResponseWrapper(app.ConversationService.Get))

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
			pr.Get("/conversations/{conversationID}/messages", wrapper.HTTPResponseWrapper(app.MessageHTTPService.History))
			pr.Post("/conversations/{conversationID}/messages", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Send))
			pr.Get("/unread", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Unread))
			pr.Patch("/messages/{messageID}", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Edit))
			pr.Delete("/messages/{messageID}", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Delete))
			pr.Post("/messages/delivery", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Delivery))
			pr.Post("/conversations/{conversationID}/read", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Read))

			// Websocket - higher rate limit to allow frequent connections
			GlobalHub = websocket.NewHub(app.MessageService)
			GlobalHub.SetSessionChecker(func(ctx context.Context, uid, sid string) bool {
				sr, ok := app.UserRepo.(repository.SessionRepository)
				if !ok {
					return false
				}
				active, err := sr.IsSessionActive(ctx, uid, sid)
				return err == nil && active
			})
			go GlobalHub.Run()

			wsHandler := wrapper.NewWebsocketHandler(GlobalHub)
			pr.Get("/ws", wsHandler.Handler)
		})
	})

	// ---------------- Health checks ----------------
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := database.GetDB().Ping(r.Context()); err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	r.Get("/redis-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.RedisClient.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	r.Get("/db-health", func(w http.ResponseWriter, r *http.Request) {
		if err := database.GetDB().Ping(r.Context()); err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	return r
}
