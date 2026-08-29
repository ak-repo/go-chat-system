package model

import (
	"database/sql"
	"time"
)

// DAO
type Message struct {
	ID               string       `json:"id" db:"id"`
	SenderID         string       `json:"sender_id" db:"sender_id"`
	ReceiverID       string       `json:"receiver_id" db:"receiver_id"`
	Body             string       `json:"content" db:"body"`
	IsGroup          bool         `json:"is_group" db:"is_group"`
	CreatedAt        time.Time    `json:"created_at" db:"created_at"`
	ModifiedAt       time.Time    `json:"modified_at,omitempty" db:"modified_at" `
	DeletedAt        sql.NullTime `json:"deleted_at,omitempty" db:"deleted_at" `
	ConversationID   string       `json:"conversation_id,omitempty" db:"conversation_id"`
	ClientMessageID  string       `json:"client_message_id,omitempty" db:"client_message_id"`
	EditedAt         *time.Time   `json:"edited_at,omitempty" db:"edited_at"`
	ReplyToMessageID string       `json:"reply_to_message_id,omitempty" db:"reply_to_message_id"`
}

type Messages []*Message

type Conversation struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	UserOneID  string    `json:"user_one_id"`
	UserTwoID  string    `json:"user_two_id"`
	CreatedAt  time.Time `json:"created_at"`
	ModifiedAt time.Time `json:"modified_at"`
}
