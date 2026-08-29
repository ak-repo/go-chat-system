package repository

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	SearchUser(ctx context.Context, filter string, limit int) (model.UsersDTO, error)
	CreateUser(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type SessionRepository interface {
	CreateSession(context.Context, *model.Session) error
	RotateSession(context.Context, []byte, *model.Session, time.Time) error
	RevokeSession(context.Context, string) error
	RevokeUserSessions(context.Context, string) error
	IsSessionActive(context.Context, string, string) (bool, error)
}

type AccountTokenRepository interface {
	CreateAccountToken(context.Context, string, string, []byte, time.Time) (string, error)
	ConsumeAccountToken(context.Context, string, []byte) (string, error)
	VerifyAccount(context.Context, []byte) (string, error)
}
type UserAccountRepository interface {
	UpdateProfile(context.Context, string, string, string) error
	ChangePassword(context.Context, string, string) error
	Deactivate(context.Context, string) error
}
type DeactivateAndRevokeRepository interface {
	DeactivateAndRevokeSessions(context.Context, string) error
}
type PasswordResetRepository interface {
	ChangePasswordAndRevokeSessions(context.Context, string, string) error
}

type UserRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewUserRepositoryImpl(db *pgxpool.Pool) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) CreateUser(ctx context.Context, user *model.User) error {
	q := `
		INSERT INTO users (
			id, username, email, password_hash, role, created_at, modified_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)

	`
	_, err := r.db.Exec(ctx, q, user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.CreatedAt, user.ModifiedAt)
	if err != nil {
		return errs.Wrap("repository.UserRepository.CreateUser", err)
	}

	return nil
}
func (r *UserRepositoryImpl) UpdateProfile(ctx context.Context, id, username, email string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET username=$2,email=$3,modified_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, username, email)
	return errs.Wrap("repository.User.UpdateProfile", err)
}
func (r *UserRepositoryImpl) ChangePassword(ctx context.Context, id, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash=$2,modified_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, hash)
	return errs.Wrap("repository.User.ChangePassword", err)
}

func (r *UserRepositoryImpl) ChangePasswordAndRevokeSessions(ctx context.Context, id, hash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Wrap("repository.User.ResetPassword", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2,modified_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, hash); err != nil {
		return errs.Wrap("repository.User.ResetPassword", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return errs.Wrap("repository.User.ResetPassword", err)
	}
	return errs.Wrap("repository.User.ResetPassword", tx.Commit(ctx))
}
func (r *UserRepositoryImpl) Deactivate(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET deleted_at=NOW(),modified_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	return errs.Wrap("repository.User.Deactivate", err)
}
func (r *UserRepositoryImpl) DeactivateAndRevokeSessions(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Wrap("repository.User.Deactivate", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE users SET deleted_at=NOW(),modified_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id); err != nil {
		return errs.Wrap("repository.User.Deactivate", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return errs.Wrap("repository.User.Deactivate", err)
	}
	return errs.Wrap("repository.User.Deactivate", tx.Commit(ctx))
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (*model.User, error) {

	var user model.User
	query := `
		SELECT id, username, email, password_hash, role
		FROM users
		WHERE lower(email) = lower($1) AND deleted_at IS NULL
	`

	err := r.db.QueryRow(ctx, query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, errs.Wrap("repository.UserRepository.GetByEmail", err)
	}
	return &user, nil
}

func (r *UserRepositoryImpl) GetByID(ctx context.Context, id string) (*model.User, error) {

	var user model.User
	query := `
		SELECT id, username, email, password_hash, role
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	err := r.db.QueryRow(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, errs.Wrap("repository.UserRepository.GetByID", err)
	}
	return &user, nil
}

func (r *UserRepositoryImpl) SearchUser(ctx context.Context, filter string, limit int) (model.UsersDTO, error) {

	if limit <= 0 {
		limit = 20
	}

	// Require minimum filter length to prevent returning all users
	if len(filter) < 2 {
		return model.UsersDTO{}, nil
	}

	var resp model.UsersDTO
	rows, err := r.db.Query(ctx, "SELECT id, username FROM users WHERE deleted_at IS NULL AND (username ILIKE $1 OR email ILIKE $1) ORDER BY lower(username), id LIMIT $2", "%"+filter+"%", limit)
	if err != nil {
		return nil, errs.Wrap("repository.UserRepository.SearchUser", err)
	}
	defer rows.Close()

	for rows.Next() {
		var user model.UserDTO
		if err := rows.Scan(&user.ID, &user.Username); err != nil {
			return nil, errs.Wrap("repository.UserRepository.SearchUser", err)
		}
		resp = append(resp, &user)
	}

	return resp, errs.Wrap("repository.UserRepository.SearchUser", rows.Err())
}

func digest(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

func (r *UserRepositoryImpl) CreateSession(ctx context.Context, s *model.Session) error {
	_, err := r.db.Exec(ctx, `INSERT INTO sessions (id,user_id,refresh_token_hash,expires_at) VALUES ($1,$2,$3,$4)`, s.ID, s.UserID, s.RefreshTokenHash, s.ExpiresAt)
	return errs.Wrap("repository.Session.Create", err)
}

// RotateSession consumes the old credential and creates its replacement in one transaction.
func (r *UserRepositoryImpl) RotateSession(ctx context.Context, oldHash []byte, newSession *model.Session, now time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Wrap("repository.Session.Rotate", err)
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM sessions WHERE refresh_token_hash=$1 AND revoked_at IS NULL AND expires_at>$2 FOR UPDATE`, oldHash, now).Scan(&userID)
	if err == pgx.ErrNoRows {
		return errs.ErrTokenReused
	}
	if err != nil {
		return errs.Wrap("repository.Session.Rotate", err)
	}
	res, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=$2,replaced_at=$2,last_used_at=$2 WHERE refresh_token_hash=$1`, oldHash, now)
	if err != nil {
		return errs.Wrap("repository.Session.Rotate", err)
	}
	if res.RowsAffected() != 1 {
		return errs.ErrTokenReused
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions (id,user_id,refresh_token_hash,expires_at) VALUES ($1,$2,$3,$4)`, newSession.ID, userID, newSession.RefreshTokenHash, newSession.ExpiresAt); err != nil {
		return errs.Wrap("repository.Session.Rotate", err)
	}
	return errs.Wrap("repository.Session.Rotate", tx.Commit(ctx))
}

func (r *UserRepositoryImpl) RevokeSession(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE id=$1 AND revoked_at IS NULL`, id)
	return errs.Wrap("repository.Session.Revoke", err)
}
func (r *UserRepositoryImpl) RevokeUserSessions(ctx context.Context, uid string) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, uid)
	return errs.Wrap("repository.Session.RevokeUser", err)
}
func (r *UserRepositoryImpl) IsSessionActive(ctx context.Context, uid, sid string) (bool, error) {
	if sid == "" {
		return false, nil
	}
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>NOW())`, sid, uid).Scan(&ok)
	return ok, errs.Wrap("repository.Session.IsActive", err)
}
func (r *UserRepositoryImpl) CreateAccountToken(ctx context.Context, uid, purpose string, hash []byte, expiry time.Time) (string, error) {
	id := uuid.NewString()
	_, err := r.db.Exec(ctx, `INSERT INTO account_tokens(id,user_id,purpose,token_hash,expires_at) VALUES($1,$2,$3,$4,$5)`, id, uid, purpose, hash, expiry)
	return id, errs.Wrap("repository.AccountToken.Create", err)
}
func (r *UserRepositoryImpl) ConsumeAccountToken(ctx context.Context, purpose string, hash []byte) (string, error) {
	var uid string
	err := r.db.QueryRow(ctx, `UPDATE account_tokens SET consumed_at=NOW() WHERE purpose=$1 AND token_hash=$2 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at>NOW() RETURNING user_id`, purpose, hash).Scan(&uid)
	if err == pgx.ErrNoRows {
		return "", errs.ErrUnauthorized
	}
	return uid, errs.Wrap("repository.AccountToken.Consume", err)
}

// VerifyAccount consumes the verification token and marks the account in the
// same transaction, so a successful token can never leave the account unverified.
func (r *UserRepositoryImpl) VerifyAccount(ctx context.Context, hash []byte) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", errs.Wrap("repository.AccountToken.Verify", err)
	}
	defer tx.Rollback(ctx)
	var uid string
	err = tx.QueryRow(ctx, `UPDATE account_tokens SET consumed_at=NOW() WHERE purpose='verification' AND token_hash=$1 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at>NOW() RETURNING user_id`, hash).Scan(&uid)
	if err == pgx.ErrNoRows {
		return "", errs.ErrUnauthorized
	}
	if err != nil {
		return "", errs.Wrap("repository.AccountToken.Verify", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET verified_at=NOW(),modified_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, uid); err != nil {
		return "", errs.Wrap("repository.AccountToken.Verify", err)
	}
	return uid, errs.Wrap("repository.AccountToken.Verify", tx.Commit(ctx))
}
