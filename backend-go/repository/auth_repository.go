package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

type AuthRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewAuthRepositoryImpl(db *pgxpool.Pool) *AuthRepositoryImpl {
	return &AuthRepositoryImpl{db: db}
}

func (r *AuthRepositoryImpl) CreateUser(ctx context.Context, user *model.User) error {
	q := `
		INSERT INTO users (
			id, username, email, password_hash, role, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)

	`
	_, err := r.db.Exec(ctx, q, user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.UpdatedAt, user.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (r *AuthRepositoryImpl) GetByEmail(ctx context.Context, email string) (*model.User, error) {

	var user model.User
	query := `
		SELECT id, username, email, password_hash, role
		FROM users
		WHERE email = $1
	`

	err := r.db.QueryRow(ctx, query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
