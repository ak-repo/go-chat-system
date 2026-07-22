package model

import (
	"database/sql"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationFriendRequest  NotificationType = "friend_request"
	NotificationFriendAccepted NotificationType = "friend_accepted"
	NotificationFriendRejected NotificationType = "friend_rejected"
	NotificationNewMessage     NotificationType = "new_message"
	NotificationUserOnline     NotificationType = "user_online"
	NotificationUserOffline    NotificationType = "user_offline"
)

// Notification represents a notification record in the database
type Notification struct {
	ID          string           `db:"id" json:"id"`
	UserID      string           `db:"user_id" json:"user_id"`
	Type        NotificationType `db:"type" json:"type"`
	Title       string           `db:"title" json:"title"`
	Body        string           `db:"body" json:"body,omitempty"`
	SenderID    string           `db:"sender_id" json:"sender_id,omitempty"`
	ReferenceID string           `db:"reference_id" json:"reference_id,omitempty"`
	IsRead      bool             `db:"is_read" json:"is_read"`
	CreatedAt   time.Time        `db:"created_at" json:"created_at"`
	DeletedAt   sql.NullTime     `db:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

// NotificationDTO is the response DTO that excludes sensitive fields
type NotificationDTO struct {
	ID          string           `db:"id" json:"id"`
	UserID      string           `db:"user_id" json:"user_id"`
	Type        NotificationType `db:"type" json:"type"`
	Title       string           `db:"title" json:"title"`
	Body        string           `db:"body" json:"body,omitempty"`
	SenderID    string           `db:"sender_id" json:"sender_id,omitempty"`
	ReferenceID string           `db:"reference_id" json:"reference_id,omitempty"`
	IsRead      bool             `db:"is_read" json:"is_read"`
	CreatedAt   time.Time        `db:"created_at" json:"created_at"`
}

// ToDTO converts Notification to NotificationDTO
func (n *Notification) ToDTO() *NotificationDTO {
	return &NotificationDTO{
		ID:          n.ID,
		UserID:      n.UserID,
		Type:        n.Type,
		Title:       n.Title,
		Body:        n.Body,
		SenderID:    n.SenderID,
		ReferenceID: n.ReferenceID,
		IsRead:      n.IsRead,
		CreatedAt:   n.CreatedAt,
	}
}

// Notifications is a slice of NotificationDTO
type Notifications []*NotificationDTO