package model

import "time"

// Message is a persisted chat message.
type Message struct {
	ID        string    `db:"id" json:"id"`
	ChatID    string    `db:"chat_id" json:"chat_id"`
	SenderID  string    `db:"sender_id" json:"sender_id"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
