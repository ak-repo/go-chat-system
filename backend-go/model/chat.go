package model

import "time"

// Chat represents a conversation (DM or group).
type Chat struct {
	ID        string    `db:"id" json:"id"`
	Type      string    `db:"type" json:"type"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ChatWithMembers is a chat with member user IDs (for listing).
type ChatWithMembers struct {
	Chat
	MemberIDs []string `json:"member_ids"`
}
