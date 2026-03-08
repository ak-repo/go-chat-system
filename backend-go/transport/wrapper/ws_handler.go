package wrapper

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"github.com/ak-repo/go-chat-system/transport/middleware"
	ws "github.com/ak-repo/go-chat-system/transport/websocket"

	"github.com/gorilla/websocket"
)

func upgrader() websocket.Upgrader {
	cfg := config.Config.WebSocket
	readBuf, writeBuf := 1024, 1024
	if cfg.MaxMessageSize > 0 {
		if cfg.MaxMessageSize < 1024 {
			readBuf = 512
			writeBuf = 512
		} else {
			readBuf = int(cfg.MaxMessageSize)
			writeBuf = int(cfg.MaxMessageSize)
		}
	}
	origins := config.Config.CORS.AllowOrigins
	if len(origins) == 0 {
		origins = []string{config.Config.CORS.Host + ":" + strconv.Itoa(config.Config.CORS.Port)}
	}
	originSet := make(map[string]bool)
	for _, o := range origins {
		originSet[strings.TrimSpace(o)] = true
	}
	return websocket.Upgrader{
		ReadBufferSize:  readBuf,
		WriteBufferSize: writeBuf,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			return originSet[origin]
		},
	}
}

type WSHandler struct {
	hub *ws.Hub
}

func NewWebsocketHandler(hub *ws.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

func (wh *WSHandler) Handler(w http.ResponseWriter, r *http.Request) {
	uid, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || uid == "" {
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := jwt.ValidateToken(tokenStr)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		uid = claims.UserID
	}

	u := upgrader()
	conn, err := u.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	if config.Config.WebSocket.MaxMessageSize > 0 {
		conn.SetReadLimit(config.Config.WebSocket.MaxMessageSize)
	}

	client := ws.NewClient(wh.hub, conn, uid, config.Config.WebSocket.ReadDeadlineSec, config.Config.WebSocket.MessagesPerSec)
	wh.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
