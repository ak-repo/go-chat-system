package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	SearchUser(ctx context.Context, filter string) (model.UsersDTO, error)
	CreateUser(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
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
	_, err := r.db.Exec(ctx, q, user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.ModifiedAt, user.CreatedAt)
	if err != nil {
		return err
	}

	return nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (*model.User, error) {

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

func (r *UserRepositoryImpl) SearchUser(ctx context.Context, filter string) (model.UsersDTO, error) {

	var resp model.UsersDTO
	rows, err := r.db.Query(ctx, "SELECT id, username, email FROM users WHERE username ILIKE $1 OR email ILIKE $1", "%"+filter+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user model.UserDTO
		if err := rows.Scan(&user.ID, &user.Username, &user.Email); err != nil {
			return nil, err
		}
		resp = append(resp, &user)
	}

	return resp, rows.Err()
}
