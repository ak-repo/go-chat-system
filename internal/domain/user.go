package domain

import (
	"context"
	"time"
)

type User struct {
	ID           string    `db:"id" json:"id"` // unique identifier
	Username     string    `db:"username" json:"username"`
	Email        string    `db:"email" json:"email"`         // email address
	PasswordHash string    `db:"password_hash" json:"-"` // hashed password
	Role         string    `db:"role" json:"role"`          // user / admin
	CreatedAt    time.Time `db:"created_at" json:"created_at"`    // creation timestamp
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`    // creation timestamp
}

type UserRepo interface {
	Create(ctx context.Context, user *User) error
	GetUser(ctx context.Context, key, value string) (*User, error)
}

type UserService interface {
	Register(ctx context.Context, username, email, password string) error
	Login(ctx context.Context, email, password string) (*User, error)
}
