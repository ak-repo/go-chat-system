package model

import (
	"database/sql"
	"time"
)

// DAO
type Friend struct {
	UserID     string
	FriendID   string
	CreatedAt  time.Time    `json:"created_at,omitempty" db:"created_at" `
	ModifiedAt time.Time    `json:"modified_at,omitempty" db:"modified_at" `
	DeletedAt  sql.NullTime `json:"deleted_at,omitempty" db:"deleted_at" `
}

type FriendDTO struct {
	UserID      string
	FriendID    string
	FriendName  string
	FriendEmail string
	CreatedAt   time.Time `json:"created_at,omitempty" db:"created_at" `
}

type FriendsDTO []*FriendDTO
