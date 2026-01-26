package repository

import (
	"context"
	"errors"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FriendRequestRepository interface {
	CreateRequest(ctx context.Context, req *model.FriendRequest) error
	GetPendingRequest(ctx context.Context, sender, receiver string) (*model.FriendRequest, error)
	GetAllRequests(ctx context.Context, userID string) (model.FriendRequestsDTO, error)

	AcceptRequest(ctx context.Context, requestID, receiverID string) error
	RejectRequest(ctx context.Context, requestID, receiverID string) error
	CancelRequest(ctx context.Context, requestID, senderID string) error
}

type FriendRequestRepositoryImpl struct {
	db *pgxpool.Pool
}

func FriendRequestRepositoryInit(db *pgxpool.Pool) *FriendRequestRepositoryImpl {
	return &FriendRequestRepositoryImpl{db: db}
}

func (r *FriendRequestRepositoryImpl) CreateRequest(ctx context.Context, req *model.FriendRequest) error {
	// Prevent duplicate pending requests both ways
	_, err := r.db.Exec(ctx, `
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

func (r *FriendRequestRepositoryImpl) GetPendingRequest(
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

func (r *FriendRequestRepositoryImpl) CancelRequest(ctx context.Context, requestID, senderID string) error {
	cmd, err := r.db.Exec(ctx, `
		DELETE FROM friend_requests
		WHERE id=$1 AND sender_id=$2 AND status='pending'
	`, requestID, senderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrRequestNotFound
	}
	return nil
}

func (r *FriendRequestRepositoryImpl) RejectRequest(ctx context.Context, requestID, receiverID string) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE friend_requests
		SET status='rejected'
		WHERE id=$1 AND receiver_id=$2 AND status='pending'
	`, requestID, receiverID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errs.ErrRequestNotFound
	}
	return nil
}

func (r *FriendRequestRepositoryImpl) AcceptRequest(ctx context.Context, requestID, receiverID string) error {
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
		return errs.ErrRequestNotFound
	}
	if err != nil {
		return err
	}

	if receiver != receiverID {
		return errs.ErrRequestNotFound
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

func (r *FriendRequestRepositoryImpl) GetAllRequests(ctx context.Context, userID string) (model.FriendRequestsDTO, error) {
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

	var resp model.FriendRequestsDTO

	for rows.Next() {
		var fr model.FriendRequestDTO
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
		resp = append(resp, &fr)
	}

	return resp, rows.Err()
}
