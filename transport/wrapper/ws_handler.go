package wrapper

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/ak-repo/go-chat-system/transport/middleware"
	ws "github.com/ak-repo/go-chat-system/transport/websocket"

	"github.com/gorilla/websocket"
)

func isWSOriginAllowed(origin string) bool {
	allowedOrigins := config.Config.CORS.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{fmt.Sprintf("%s:%d", config.Config.CORS.Host, config.Config.CORS.Port)}
	}

	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // same-origin requests
		}
		return isWSOriginAllowed(origin)
	},
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
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	client := ws.NewClient(wh.hub, conn, uid)
	wh.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
