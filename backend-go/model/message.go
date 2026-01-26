package model

import (
	"database/sql"
	"time"
)

// DAO
type Message struct {
	ID         string       `json:"id"`
	SenderID   string       `json:"sender_id"`
	ReceiverID string       `json:"receiver_id"`
	Body       string       `json:"content"`
	IsGroup    bool         `josn:"is_group"`
	CreatedAt  time.Time    `json:"created_at"`
	ModifiedAt time.Time    `json:"modified_at,omitempty" db:"modified_at" `
	DeletedAt  sql.NullTime `json:"deleted_at,omitempty" db:"deleted_at" `
}
