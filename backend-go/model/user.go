package model

import (
	"database/sql"
	"time"
)

// DAO
type User struct {
	ID           string       `db:"id" json:"id"` // unique identifier
	Username     string       `db:"username" json:"username"`
	Email        string       `db:"email" json:"email"`     // email address
	PasswordHash string       `db:"password_hash" json:"-"` // hashed password
	Role         string       `db:"role" json:"role"`       // user / admin
	CreatedAt    time.Time    `json:"created_at,omitempty" db:"created_at" `
	ModifiedAt   time.Time    `json:"modified_at,omitempty" db:"modified_at" `
	DeletedAt    sql.NullTime `json:"deleted_at,omitempty" db:"deleted_at" `
}

type UserDTO struct {
	ID       string `db:"id" json:"id"` // unique identifier
	Username string `db:"username" json:"username"`
	Email    string `db:"email" json:"email"`
	Role     string `db:"role" json:"role"`
}

type UsersDTO []*UserDTO
