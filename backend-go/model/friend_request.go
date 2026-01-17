package model

import "time"

type FriendRequestStatus string

const (
	FriendPending  FriendRequestStatus = "pending"
	FriendAccepted FriendRequestStatus = "accepted"
	FriendBlocked  FriendRequestStatus = "blocked"
)

type FriendRequest struct {
	ID         string
	SenderID   string
	ReceiverID string
	Status     FriendRequestStatus
	CreatedAt  time.Time
}
