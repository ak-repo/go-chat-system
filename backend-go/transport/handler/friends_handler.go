package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ak-repo/go-chat-system/service"
	"github.com/ak-repo/go-chat-system/transport/middleware"
)

type FriendHandler struct {
	friendService service.FriendService
}

func NewFriendHandler(fs service.FriendService) *FriendHandler {
	return &FriendHandler{friendService: fs}
}

// POST /friends/request
func (h *FriendHandler) SendRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		To string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.friendService.SendRequest(r.Context(), userID, body.To); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// POST /friends/accept
func (h *FriendHandler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequestID string `json:"request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.RequestID == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}

	if err := h.friendService.AcceptRequest(r.Context(), body.RequestID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GET /friends
func (h *FriendHandler) ListFriends(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	friends, err := h.friendService.ListFriends(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := struct {
		Friends []string `json:"friends"`
	}{
		Friends: friends,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// POST /friends/block
func (h *FriendHandler) BlockUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.friendService.BlockUser(r.Context(), userID, body.Target); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
