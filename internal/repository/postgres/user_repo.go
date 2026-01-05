package postgres

import (
	"context"
	"fmt"

	"github.com/ak-repo/go-chat-system/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) domain.UserRepo {
	return &userRepo{pool: pool}
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {

	q := `
		INSERT INTO users (id, username, email, password_hash,role,created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, q,
		user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *userRepo) GetUser(ctx context.Context, key, value string) (*domain.User, error) {
	allowedKeys := map[string]bool{
		"id":       true,
		"email":    true,
		"username": true,
	}
	if !allowedKeys[key] {
		return nil, fmt.Errorf("invalid lookup key: %s", key)

	}

	user := &domain.User{}
	q := `
		SELECT
			id,
			username,
			email,
			password_hash,
			role,
			created_at,
			updated_at
		FROM users
		WHERE ` + key + ` = $1
	`
	err := r.pool.QueryRow(ctx, q, value).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {

		return nil, err
	}
	return user, nil
}
