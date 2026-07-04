package model

import (
	"database/sql"
	"time"
)

type FriendRequestStatus string

const (
	FriendPending  FriendRequestStatus = "pending"
	FriendAccepted FriendRequestStatus = "accepted"
	FriendRejected FriendRequestStatus = "rejected"
	FriendBlocked  FriendRequestStatus = "blocked"
)

// DAO -> DB representation
type FriendRequest struct {
	ID         string
	SenderID   string
	ReceiverID string
	Status     FriendRequestStatus
	CreatedAt  time.Time    `json:"created_at,omitempty" db:"created_at" `
	ModifiedAt time.Time    `json:"modified_at,omitempty" db:"modified_at" `
	DeletedAt  sql.NullTime `json:"deleted_at,omitempty" db:"deleted_at" `
}

// DTO's
type FriendRequestDTO struct {
	ID          string
	SenderID    string
	ReceiverID  string
	FriendName  string
	FriendEmail string
	Status      FriendRequestStatus
	CreatedAt   time.Time `json:"created_at,omitempty" db:"created_at" `
}

type FriendRequestsDTO []*FriendRequestDTO
