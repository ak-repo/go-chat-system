package http

import (
	"log"
	"net/http"

	ws_pkg "github.com/ak-repo/go-chat-system/internal/transport/websocket"
	"github.com/ak-repo/go-chat-system/pkg/helper"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // tighten for production
	},
}

func (h *Handler) wsHandler(w http.ResponseWriter, r *http.Request) {

	uid, ok := helper.UserIDFromContext(r.Context())
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

	client := ws_pkg.NewClient(h.hub, conn, uid)
	h.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
