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
				if err != nil || u == nil || u.VerifiedAt == nil {
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
			pr.Get("/users/{userID}", wrapper.HTTPResponseWrapper(app.UserService.GetPublicUser))
			pr.Patch("/users/me", wrapper.HTTPResponseWrapper(app.UserService.UpdateMe))
			pr.Post("/users/me/change-password", wrapper.HTTPResponseWrapper(app.UserService.ChangePassword))
			pr.Delete("/users/me", wrapper.HTTPResponseWrapper(app.UserService.Deactivate))
			pr.Post("/conversations", wrapper.HTTPResponseWrapper(app.ConversationService.Create))
			pr.Get("/conversations", wrapper.HTTPResponseWrapper(app.ConversationService.List))
			pr.Get("/conversations/{conversationID}", wrapper.HTTPResponseWrapper(app.ConversationService.Get))
			pr.Patch("/conversations/{conversationID}/preferences", wrapper.HTTPResponseWrapper(app.ConversationService.Preferences))
			pr.Delete("/conversations/{conversationID}", wrapper.HTTPResponseWrapper(app.ConversationService.Delete))
			pr.Post("/groups", wrapper.HTTPResponseWrapper(app.GroupConversationService.CreateGroup))
			pr.Get("/groups/{conversationID}", wrapper.HTTPResponseWrapper(app.GroupConversationService.GetGroup))
			pr.Patch("/groups/{conversationID}", wrapper.HTTPResponseWrapper(app.GroupConversationService.UpdateGroup))
			pr.Get("/groups/{conversationID}/members", wrapper.HTTPResponseWrapper(app.GroupConversationService.ListGroupMembers))
			pr.Post("/groups/{conversationID}/members", wrapper.HTTPResponseWrapper(app.GroupConversationService.AddGroupMembers))
			pr.Delete("/groups/{conversationID}/members/{userID}", wrapper.HTTPResponseWrapper(app.GroupConversationService.RemoveGroupMember))
			pr.Post("/groups/{conversationID}/leave", wrapper.HTTPResponseWrapper(app.GroupConversationService.LeaveGroup))
			pr.Patch("/groups/{conversationID}/members/{userID}/role", wrapper.HTTPResponseWrapper(app.GroupConversationService.UpdateGroupMemberRole))
			pr.Post("/groups/{conversationID}/invites", wrapper.HTTPResponseWrapper(app.GroupConversationService.CreateGroupInvite))
			pr.Get("/groups/{conversationID}/invites", wrapper.HTTPResponseWrapper(app.GroupConversationService.ListGroupInvites))
			pr.Delete("/groups/{conversationID}/invites/{inviteID}", wrapper.HTTPResponseWrapper(app.GroupConversationService.RevokeGroupInvite))
			pr.Post("/group-invites/{token}/accept", wrapper.HTTPResponseWrapper(app.GroupConversationService.AcceptGroupInvite))

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
			pr.Post("/conversations/{conversationID}/messages/{messageID}/forward", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Forward))
			pr.Get("/unread", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Unread))
			pr.Post("/unread/read-all", wrapper.HTTPResponseWrapper(app.MessageHTTPService.ReadAll))
			pr.Patch("/messages/{messageID}", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Edit))
			pr.Delete("/messages/{messageID}", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Delete))
			pr.Put("/messages/{messageID}/reactions/{reaction}", wrapper.HTTPResponseWrapper(app.MessageHTTPService.AddReaction))
			pr.Delete("/messages/{messageID}/reactions/{reaction}", wrapper.HTTPResponseWrapper(app.MessageHTTPService.RemoveReaction))
			pr.Post("/messages/delivery", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Delivery))
			pr.Post("/conversations/{conversationID}/read", wrapper.HTTPResponseWrapper(app.MessageHTTPService.Read))
			pr.Get("/notifications", wrapper.HTTPResponseWrapper(app.NotificationService.List))
			pr.Get("/notifications/unread", wrapper.HTTPResponseWrapper(app.NotificationService.Unread))
			pr.Post("/notifications/{notificationID}/read", wrapper.HTTPResponseWrapper(app.NotificationService.Read))
			pr.Post("/notifications/read-all", wrapper.HTTPResponseWrapper(app.NotificationService.ReadAll))
			pr.Get("/notification-preferences", wrapper.HTTPResponseWrapper(app.NotificationService.Preferences))
			pr.Patch("/notification-preferences", wrapper.HTTPResponseWrapper(app.NotificationService.Preferences))

			// Websocket - higher rate limit to allow frequent connections
			GlobalHub = websocket.NewHub(app.MessageService, app.PresenceService)
			app.NotificationService.SetPublisher(GlobalHub.Publish)
			GlobalHub.SetSessionChecker(func(ctx context.Context, uid, sid string) bool {
				u, userErr := app.UserRepo.GetByID(ctx, uid)
				if userErr != nil || u == nil || u.VerifiedAt == nil {
					return false
				}
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
