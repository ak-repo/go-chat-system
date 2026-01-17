package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FriendRepository interface {
	CreateRequest(ctx context.Context, req *model.FriendRequest) error
	GetPendingRequest(ctx context.Context, sender, receiver string) (*model.FriendRequest, error)

	AcceptRequest(ctx context.Context, requestID, receiverID string) error
	RejectRequest(ctx context.Context, requestID, receiverID string) error
	CancelRequest(ctx context.Context, requestID, senderID string) error

	CreateFriendship(ctx context.Context, a, b string) error
	AreFriends(ctx context.Context, a, b string) (bool, error)
	ListFriends(ctx context.Context, userID string) ([]*model.Friend, error)

	BlockUser(ctx context.Context, blocker, target string) error
	UnblockUser(ctx context.Context, blocker, target string) error
	IsBlocked(ctx context.Context, a, b string) (bool, error)

	GetAllRequests(ctx context.Context, userID string) ([]model.ListFriendRequest, error)
}

var (
	ErrSelfAction          = errors.New("cannot perform this action on yourself")
	ErrRequestNotFound     = errors.New("friend request not found")
	ErrNotAuthorized       = errors.New("not authorized to perform this action")
	ErrAlreadyFriends      = errors.New("users are already friends")
	ErrBlockedRelationship = errors.New("one of the users has blocked the other")
)

type FriendRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewFriendRepositoryImpl(db *pgxpool.Pool) *FriendRepositoryImpl {
	return &FriendRepositoryImpl{db: db}
}

//
// Helpers
//

func (r *FriendRepositoryImpl) IsBlocked(ctx context.Context, a, b string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM blocks
			WHERE (blocker_id=$1 AND blocked_id=$2)
			   OR (blocker_id=$2 AND blocked_id=$1)
		)
	`, a, b).Scan(&exists)
	return exists, err
}

//
// Friend Requests
//

func (r *FriendRepositoryImpl) CreateRequest(ctx context.Context, req *model.FriendRequest) error {
	if req.SenderID == req.ReceiverID {
		return ErrSelfAction
	}

	blocked, err := r.IsBlocked(ctx, req.SenderID, req.ReceiverID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlockedRelationship
	}

	already, err := r.AreFriends(ctx, req.SenderID, req.ReceiverID)
	if err != nil {
		return err
	}
	if already {
		return ErrAlreadyFriends
	}

	// Prevent duplicate pending requests both ways
	_, err = r.db.Exec(ctx, `
		INSERT INTO friend_requests (id, sender_id, receiver_id, status, created_at)
		SELECT $1, $2, $3, $4, $5
		WHERE NOT EXISTS (
			SELECT 1
			FROM friend_requests
			WHERE (
				(sender_id=$2 AND receiver_id=$3)
			 OR (sender_id=$3 AND receiver_id=$2)
			)
			AND status='pending'
		)
	`, req.ID, req.SenderID, req.ReceiverID, req.Status, req.CreatedAt)

	return err
}

func (r *FriendRepositoryImpl) GetPendingRequest(
	ctx context.Context,
	sender, receiver string,
) (*model.FriendRequest, error) {

	row := r.db.QueryRow(ctx, `
		SELECT id, sender_id, receiver_id, status, created_at
		FROM friend_requests
		WHERE sender_id=$1
		  AND receiver_id=$2
		  AND status='pending'
	`, sender, receiver)

	var fr model.FriendRequest
	err := row.Scan(&fr.ID, &fr.SenderID, &fr.ReceiverID, &fr.Status, &fr.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &fr, nil
}

func (r *FriendRepositoryImpl) CancelRequest(ctx context.Context, requestID, senderID string) error {
	cmd, err := r.db.Exec(ctx, `
		DELETE FROM friend_requests
		WHERE id=$1 AND sender_id=$2 AND status='pending'
	`, requestID, senderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrRequestNotFound
	}
	return nil
}

func (r *FriendRepositoryImpl) RejectRequest(ctx context.Context, requestID, receiverID string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE friend_requests
		SET status='rejected'
		WHERE id=$1 AND receiver_id=$2 AND status='pending'
	`, requestID, receiverID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrRequestNotFound
	}
	return nil
}

func (r *FriendRepositoryImpl) AcceptRequest(ctx context.Context, requestID, receiverID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var sender, receiver string

	// Lock row to avoid race conditions
	err = tx.QueryRow(ctx, `
		SELECT sender_id, receiver_id
		FROM friend_requests
		WHERE id=$1 AND status='pending'
		FOR UPDATE
	`, requestID).Scan(&sender, &receiver)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRequestNotFound
	}
	if err != nil {
		return err
	}

	if receiver != receiverID {
		return ErrNotAuthorized
	}

	// block check
	blocked, err := r.isBlockedTx(ctx, tx, sender, receiver)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlockedRelationship
	}

	// already friends check
	already, err := r.areFriendsTx(ctx, tx, sender, receiver)
	if err != nil {
		return err
	}
	if already {
		// request can be marked accepted anyway to keep data consistent
		_, _ = tx.Exec(ctx, `
			UPDATE friend_requests
			SET status='accepted'
			WHERE id=$1
		`, requestID)
		return tx.Commit(ctx)
	}

	// mark accepted
	_, err = tx.Exec(ctx, `
		UPDATE friend_requests
		SET status='accepted'
		WHERE id=$1 AND status='pending'
	`, requestID)
	if err != nil {
		return err
	}

	// Create mutual friendship (idempotent)
	_, err = tx.Exec(ctx, `
		INSERT INTO friends (user_id, friend_id)
		VALUES ($1, $2), ($2, $1)
		ON CONFLICT DO NOTHING
	`, sender, receiver)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

//
// Friendship
//

func (r *FriendRepositoryImpl) CreateFriendship(ctx context.Context, a, b string) error {
	if a == b {
		return ErrSelfAction
	}

	blocked, err := r.IsBlocked(ctx, a, b)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlockedRelationship
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO friends (user_id, friend_id)
		VALUES ($1, $2), ($2, $1)
		ON CONFLICT DO NOTHING
	`, a, b)

	return err
}

func (r *FriendRepositoryImpl) AreFriends(ctx context.Context, a, b string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM friends
			WHERE user_id=$1 AND friend_id=$2
		)
	`, a, b).Scan(&exists)

	return exists, err
}

func (r *FriendRepositoryImpl) ListFriends(ctx context.Context, userID string) ([]*model.Friend, error) {
	rows, err := r.db.Query(ctx, `
		SELECT f.user_id,
			   f.friend_id,
			   u.name,
			   u.email,
			   f.since
		FROM friends f
		JOIN users u ON u.id = f.friend_id
		WHERE f.user_id=$1
		ORDER BY f.since DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends []*model.Friend

	for rows.Next() {
		var f model.Friend
		if err := rows.Scan(&f.UserID, &f.FriendID, &f.FriendName, &f.FriendEmail, &f.Since); err != nil {
			return nil, err
		}
		friends = append(friends, &f)
	}

	return friends, rows.Err()
}

//
// Blocking
//

func (r *FriendRepositoryImpl) BlockUser(ctx context.Context, blocker, target string) error {
	if blocker == target {
		return ErrSelfAction
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert block
	_, err = tx.Exec(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, blocker, target)
	if err != nil {
		return err
	}

	// Remove friendships both ways
	_, err = tx.Exec(ctx, `
		DELETE FROM friends
		WHERE (user_id=$1 AND friend_id=$2)
		   OR (user_id=$2 AND friend_id=$1)
	`, blocker, target)
	if err != nil {
		return err
	}

	// Update requests to blocked
	_, err = tx.Exec(ctx, `
		UPDATE friend_requests
		SET status='blocked'
		WHERE (sender_id=$1 AND receiver_id=$2)
		   OR (sender_id=$2 AND receiver_id=$1)
	`, blocker, target)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *FriendRepositoryImpl) UnblockUser(ctx context.Context, blocker, target string) error {
	cmd, err := r.db.Exec(ctx, `
		DELETE FROM blocks
		WHERE blocker_id=$1 AND blocked_id=$2
	`, blocker, target)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("block relationship not found")
	}
	return nil
}

//
// Requests listing (receiver inbox)
//

func (r *FriendRepositoryImpl) GetAllRequests(ctx context.Context, userID string) ([]model.ListFriendRequest, error) {
	query := `
		SELECT fr.id,
			   fr.sender_id,
			   fr.receiver_id,
			   fr.status,
			   u.name,
			   u.email,
			   fr.created_at
		FROM friend_requests fr
		JOIN users u ON u.id = fr.sender_id
		WHERE fr.receiver_id=$1
		ORDER BY fr.created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var resp []model.ListFriendRequest

	for rows.Next() {
		var fr model.ListFriendRequest
		if err := rows.Scan(
			&fr.ID,
			&fr.SenderID,
			&fr.ReceiverID,
			&fr.Status,
			&fr.FriendName,
			&fr.FriendEmail,
			&fr.CreatedAt,
		); err != nil {
			return nil, err
		}
		resp = append(resp, fr)
	}

	return resp, rows.Err()
}

//
// Tx helpers
//

func (r *FriendRepositoryImpl) isBlockedTx(ctx context.Context, tx pgx.Tx, a, b string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM blocks
			WHERE (blocker_id=$1 AND blocked_id=$2)
			   OR (blocker_id=$2 AND blocked_id=$1)
		)
	`, a, b).Scan(&exists)
	return exists, err
}

func (r *FriendRepositoryImpl) areFriendsTx(ctx context.Context, tx pgx.Tx, a, b string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM friends
			WHERE user_id=$1 AND friend_id=$2
		)
	`, a, b).Scan(&exists)
	return exists, err
}
