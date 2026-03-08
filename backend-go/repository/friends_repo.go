package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FriendRepository interface {
	CreateFriendship(ctx context.Context, a, b string) error
	AreFriends(ctx context.Context, a, b string) (bool, error)
	ListFriends(ctx context.Context, userID string, limit, offset int) (model.FriendsDTO, error)
}

type FriendRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewFriendRepositoryImpl(db *pgxpool.Pool) *FriendRepositoryImpl {
	return &FriendRepositoryImpl{db: db}
}

func (r *FriendRepositoryImpl) CreateFriendship(ctx context.Context, a, b string) error {

	_, err := r.db.Exec(ctx, `
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

func (r *FriendRepositoryImpl) ListFriends(ctx context.Context, userID string, limit, offset int) (model.FriendsDTO, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.Query(ctx, `
		SELECT f.user_id,
			   f.friend_id,
			   u.username,
			   u.email,
			   f.created_at
		FROM friends f
		JOIN users u ON u.id = f.friend_id
		WHERE f.user_id=$1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var friends model.FriendsDTO

	for rows.Next() {
		var f model.FriendDTO
		if err := rows.Scan(&f.UserID, &f.FriendID, &f.FriendName, &f.FriendEmail, &f.CreatedAt); err != nil {
			return nil, err
		}
		friends = append(friends, &f)
	}

	return friends, rows.Err()
}
