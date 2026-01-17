package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	SearchUser(ctx context.Context, filter string) ([]model.SearchUser, error)
}

type UserRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewUserRepositoryImpl(db *pgxpool.Pool) *UserRepositoryImpl {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) SearchUser(ctx context.Context, filter string) ([]model.SearchUser, error) {

	var resp []model.SearchUser
	rows, err := r.db.Query(ctx, "SELECT id, username, email FROM users WHERE username ILIKE $1 OR email ILIKE $1", "%"+filter+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user model.SearchUser
		if err := rows.Scan(&user.ID, &user.Username, &user.Email); err != nil {
			return nil, err
		}
		resp = append(resp, user)
	}

	return resp, rows.Err()
}
