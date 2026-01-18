package model

import "time"

type FriendRequestStatus string

const (
	FriendPending  FriendRequestStatus = "pending"
	FriendAccepted FriendRequestStatus = "accepted"
	FriendRejected FriendRequestStatus = "rejected"
	FriendBlocked  FriendRequestStatus = "blocked"
)

type FriendRequest struct {
	ID         string
	SenderID   string
	ReceiverID string
	Status     FriendRequestStatus
	CreatedAt  time.Time
}

type Friend struct {
	UserID      string
	FriendID    string
	FriendName  string
	FriendEmail string
	Since       time.Time
}

type ListFriendRequest struct {
	ID          string
	SenderID    string
	ReceiverID  string
	Status      FriendRequestStatus
	FriendName  string
	FriendEmail string
	CreatedAt   time.Time
}
