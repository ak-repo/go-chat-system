package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/jwt"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/google/uuid"
)

// TODO : X no method wrapping

func Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		utils.ErrorResponse(w, "invalid request payload", err, http.StatusBadRequest, r)
		return
	}
	if dec.More() {
		utils.ErrorResponse(w, "unexpected extra data", nil, http.StatusBadRequest, r)
		return
	}

	if !utils.Required(req.Username) ||
		!utils.Required(req.Email) ||
		!utils.Required(req.Password) {
		utils.ErrorResponse(w, "missing required fields", nil, http.StatusBadRequest, r)
		return
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.ErrorResponse(w, "password hashing failed", err, http.StatusInternalServerError, r)
		return
	}

	now := time.Now().UTC()
	userID := uuid.NewString()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO users (
			id, username, email, password_hash, role, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = database.DB.Exec(
		ctx,
		query,
		userID,
		req.Username,
		req.Email,
		hash,
		"user",
		now,
		now,
	)

	if err != nil {
		if utils.IsUniqueViolation(err) {
			utils.ErrorResponse(w, "user already exists", nil, http.StatusConflict, r)
			return
		}
		utils.ErrorResponse(w, "database insert failed", err, http.StatusInternalServerError, r)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(utils.SuccessResponse(map[string]interface{}{
		"message": "registration successful",
		"user_id": userID,
	}))
}

func UserLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		utils.ErrorResponse(w, "invalid request payload", err, http.StatusBadRequest, r)
		return
	}

	if !utils.Required(req.Email) || !utils.Required(req.Password) {
		utils.ErrorResponse(w, "email and password required", nil, http.StatusBadRequest, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var user model.User
	query := `
		SELECT id, username, email, password_hash, role
		FROM users
		WHERE email = $1
	`

	err := database.DB.QueryRow(ctx, query, req.Email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role)

	if err != nil {
		utils.ErrorResponse(w, "invalid credentials", nil, http.StatusUnauthorized, r)
		return
	}

	if !utils.ComparePassword(user.PasswordHash, req.Password) {
		utils.ErrorResponse(w, "invalid credentials", nil, http.StatusUnauthorized, r)
		return
	}

	token, ttl, err := jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		utils.ErrorResponse(w, "token generation failed", err, http.StatusInternalServerError, r)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Expires:  ttl,
	})

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(utils.SuccessResponse(map[string]interface{}{
		"message": "login successful",
		"user":    user,
		"token":   token,
	}))
}
