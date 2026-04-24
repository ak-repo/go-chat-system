package repository

import (
	"context"
	"fmt"

	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BlockRepository interface {
	BlockUser(ctx context.Context, blocker, target string) error
	UnblockUser(ctx context.Context, blocker, target string) error
	IsBlocked(ctx context.Context, a, b string) (bool, error)
}

type BlockRepositoryImpl struct {
	db *pgxpool.Pool
}

func BlockRepositoryInit(db *pgxpool.Pool) *BlockRepositoryImpl {
	return &BlockRepositoryImpl{db: db}
}

func (r *BlockRepositoryImpl) IsBlocked(ctx context.Context, a, b string) (bool, error) {
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

func (r *BlockRepositoryImpl) BlockUser(ctx context.Context, blocker, target string) error {
	if blocker == target {
		return errs.ErrSelfAction
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

func (r *BlockRepositoryImpl) UnblockUser(ctx context.Context, blocker, target string) error {
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
