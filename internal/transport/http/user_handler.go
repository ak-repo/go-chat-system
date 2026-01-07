package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/pkg/utils"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !utils.Required(req.Email) || !utils.Required(req.Password) || !utils.Required(req.Username) {
		http.Error(w, "invalid inputes", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.userService.Register(ctx, req.Username, req.Email, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"error":   nil,
	})

}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		log.Println("err: ", err.Error())
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !utils.Required(req.Email) || !utils.Required(req.Password) {
		http.Error(w, "invalid inputes", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	token, ttl, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println(int(ttl.Hour()))

	//Cookie
	cookie := &http.Cookie{
		Name:     "access",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(ttl.Second()),
	}
	http.SetCookie(w, cookie)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"user":    user,
	})

}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("home"))

}
