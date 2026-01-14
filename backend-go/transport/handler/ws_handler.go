package handler

import (
	"log"
	"net/http"

	"github.com/ak-repo/go-chat-system/transport/middleware"
	ws "github.com/ak-repo/go-chat-system/transport/websocket"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // tighten for production
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
