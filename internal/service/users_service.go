package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/jwt"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/google/uuid"
)

type UserService interface {
	SearchUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	Register(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	Login(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	RefreshToken(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	Logout(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	GetMe(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	UpdateMe(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	ChangePassword(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	Deactivate(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)

	//TODO: admin actions

}

func newRefresh(userID string) (string, time.Time, *model.Session, error) {
	sessionID := uuid.NewString()
	token, expiry, err := jwt.GenerateRefreshTokenForSession(userID, sessionID)
	if err != nil {
		return "", time.Time{}, nil, err
	}
	h := sha256.Sum256([]byte(token))
	return token, expiry, &model.Session{ID: sessionID, UserID: userID, RefreshTokenHash: h[:], ExpiresAt: expiry}, nil
}
func configRefreshExpiry() time.Duration {
	d := jwt.RefreshExpiry()
	if d <= 0 {
		return 7 * 24 * time.Hour
	}
	return d
}

type UserServiceImpl struct {
	userRepo repository.UserRepository
}

func NewUserServiceImpl(userRepo repository.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{userRepo: userRepo}
}

func (s *UserServiceImpl) SearchUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	filter := r.URL.Query().Get("filter")
	if len(filter) > 128 {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	respObj, err := s.userRepo.SearchUser(r.Context(), filter, limit)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.SearchUser", err)
	}

	responseData := map[string]any{
		"users": respObj,
	}
	return http.StatusOK, utils.SuccessResponse(responseData), nil
}

func (s *UserServiceImpl) Register(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		return http.StatusBadRequest, nil, errs.Wrap("service.UserService.Register", err)
	}

	if !utils.Required(req.Username) ||
		!utils.Required(req.Email) ||
		!utils.Required(req.Password) {

		return http.StatusBadRequest, nil, errs.ErrValidation
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if len(req.Username) > utils.MaxUsernameLength || len(req.Email) > utils.MaxEmailLength {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	// Validate email format
	if !utils.ValidateEmail(req.Email) {
		return http.StatusBadRequest, nil, errs.ErrInvalidEmail
	}

	// Validate password minimum length
	if !utils.ValidatePassword(req.Password) {
		return http.StatusBadRequest, nil, errs.ErrWeakPassword
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {

		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Register", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         "user",
		CreatedAt:    time.Now().UTC(),
		ModifiedAt:   time.Now().UTC(),
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Register", err)
	}

	refreshToken, refreshTTL, session, err := newRefresh(user.ID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Register", err)
	}
	if sr, ok := s.userRepo.(repository.SessionRepository); ok {
		if err := sr.CreateSession(r.Context(), session); err != nil {
			return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Register", err)
		}
	}
	token, ttl, err := jwt.GenerateTokenForSession(user.ID, user.Email, user.Role, session.ID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Register", err)
	}
	responseData := map[string]any{
		"user": &model.UserDTO{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
		"token":         token,
		"exp":           ttl,
		"refresh_token": refreshToken, "refresh_exp": refreshTTL,
	}
	return http.StatusCreated, utils.SuccessResponse(responseData), nil

}

func (s *UserServiceImpl) Login(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		return http.StatusBadRequest, nil, errs.Wrap("service.UserService.Login", err)
	}

	if !utils.Required(req.Email) || !utils.Required(req.Password) {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if len(req.Email) > utils.MaxEmailLength || len(req.Password) > utils.MaxPasswordLength {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Login", err)
	}
	if user == nil {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized

	}

	if !utils.ComparePassword(user.PasswordHash, req.Password) {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	user.PasswordHash = ""

	refreshToken, refreshTTL, session, err := newRefresh(user.ID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Login", err)
	}

	if sr, ok := s.userRepo.(repository.SessionRepository); ok {
		if err := sr.CreateSession(ctx, session); err != nil {
			return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Login", err)
		}
	}
	token, ttl, err := jwt.GenerateTokenForSession(user.ID, user.Email, user.Role, session.ID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.Login", err)
	}
	responseData := map[string]any{
		"user": &model.UserDTO{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Role:     user.Role,
		},
		"token":         token,
		"exp":           ttl,
		"refresh_token": refreshToken,
		"refresh_exp":   refreshTTL,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil

}

func (s *UserServiceImpl) RefreshToken(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	if !utils.Required(req.RefreshToken) {
		return http.StatusBadRequest, nil, errs.ErrValidation
	}

	claims, err := jwt.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil || user == nil {
		if err != nil {
			return http.StatusUnauthorized, nil, errs.Wrap("service.UserService.RefreshToken", err)
		}
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}

	newToken, refreshTTL, session, err := newRefresh(user.ID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.RefreshToken", err)
	}
	if sr, ok := s.userRepo.(repository.SessionRepository); ok {
		h := sha256.Sum256([]byte(req.RefreshToken))
		if err := sr.RotateSession(ctx, h[:], session, time.Now().UTC()); err != nil {
			return http.StatusUnauthorized, nil, err
		}
	}
	token, ttl, err := jwt.GenerateTokenForSession(user.ID, user.Email, user.Role, session.ID)
	if err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.UserService.RefreshToken", err)
	}

	responseData := map[string]any{
		"token":         token,
		"exp":           ttl,
		"refresh_token": newToken,
		"refresh_exp":   refreshTTL,
	}

	return http.StatusOK, utils.SuccessResponse(responseData), nil
}

func actor(r *http.Request) (string, error) {
	id, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || id == "" {
		return "", errs.ErrUnauthorized
	}
	return id, nil
}
func (s *UserServiceImpl) Logout(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	id, err := actor(r)
	if err != nil {
		return 401, nil, err
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil || req.RefreshToken == "" {
		return 400, nil, errs.ErrValidation
	}
	claims, e := jwt.ValidateRefreshToken(req.RefreshToken)
	if e != nil || claims.UserID != id || claims.SessionID == "" {
		return 401, nil, errs.ErrUnauthorized
	}
	if sr, ok := s.userRepo.(repository.SessionRepository); ok {
		if e = sr.RevokeSession(r.Context(), claims.SessionID); e != nil {
			return 500, nil, errs.Wrap("service.User.Logout", e)
		}
	} else {
		return 500, nil, errs.ErrInternal
	}
	return http.StatusOK, utils.SuccessResponse(map[string]any{"user_id": id}), nil
}
func (s *UserServiceImpl) GetMe(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	id, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	u, e := s.userRepo.GetByID(r.Context(), id)
	if e != nil {
		return 500, nil, errs.Wrap("service.User.GetMe", e)
	}
	if u == nil {
		return 401, nil, errs.ErrInactiveUser
	}
	return 200, utils.SuccessResponse(&model.UserDTO{ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role}), nil
}
func (s *UserServiceImpl) UpdateMe(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	id, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || !utils.Required(q.Username) || !utils.ValidateEmail(q.Email) {
		return 400, nil, errs.ErrValidation
	}
	q.Username = strings.TrimSpace(q.Username)
	q.Email = strings.ToLower(strings.TrimSpace(q.Email))
	if len(q.Username) > utils.MaxUsernameLength || len(q.Email) > utils.MaxEmailLength {
		return 400, nil, errs.ErrValidation
	}
	a, ok := s.userRepo.(repository.UserAccountRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	if e = a.UpdateProfile(r.Context(), id, q.Username, q.Email); e != nil {
		return 500, nil, errs.Wrap("service.User.UpdateMe", e)
	}
	return s.GetMe(w, r)
}
func (s *UserServiceImpl) ChangePassword(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	id, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	var q struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&q) != nil || !utils.ValidatePassword(q.Password) {
		return 400, nil, errs.ErrWeakPassword
	}
	h, e := utils.HashPassword(q.Password)
	if e != nil {
		return 500, nil, e
	}
	a, ok := s.userRepo.(repository.UserAccountRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	if pr, atomic := s.userRepo.(repository.PasswordResetRepository); atomic {
		// The concrete repository performs password update and revocation in one transaction.
		if e = pr.ChangePasswordAndRevokeSessions(r.Context(), id, h); e != nil {
			return 500, nil, errs.Wrap("service.User.ChangePassword", e)
		}
	} else {
		if e = a.ChangePassword(r.Context(), id, h); e != nil {
			return 500, nil, e
		}
		if sr, ok := s.userRepo.(repository.SessionRepository); ok {
			if e = sr.RevokeUserSessions(r.Context(), id); e != nil {
				return 500, nil, errs.Wrap("service.User.ChangePassword", e)
			}
		}
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "changed"}), nil
}
func (s *UserServiceImpl) Deactivate(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	id, e := actor(r)
	if e != nil {
		return 401, nil, e
	}
	a, ok := s.userRepo.(repository.UserAccountRepository)
	if !ok {
		return 500, nil, errs.ErrInternal
	}
	if ar, atomic := s.userRepo.(repository.DeactivateAndRevokeRepository); atomic {
		if e = ar.DeactivateAndRevokeSessions(r.Context(), id); e != nil {
			return 500, nil, errs.Wrap("service.User.Deactivate", e)
		}
	} else {
		if e = a.Deactivate(r.Context(), id); e != nil {
			return 500, nil, e
		}
		if sr, ok := s.userRepo.(repository.SessionRepository); ok {
			if e = sr.RevokeUserSessions(r.Context(), id); e != nil {
				return 500, nil, errs.Wrap("service.User.Deactivate", e)
			}
		}
	}
	return 200, utils.SuccessResponse(map[string]string{"status": "deactivated"}), nil
}
