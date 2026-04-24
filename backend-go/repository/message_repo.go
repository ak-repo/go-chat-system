package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository interface {
	CreateMessage(ctx context.Context, msg *model.Message) error
	GetMessagesByReceiver(ctx context.Context, receiverID string, limit, offset int) (model.Messages, error)
	GetMessagesBetweenUsers(ctx context.Context, senderID, receiverID string, limit, offset int) (model.Messages, error)
}

type MessageRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewMessageRepositoryImpl(db *pgxpool.Pool) *MessageRepositoryImpl {
	return &MessageRepositoryImpl{db: db}
}

func (r *MessageRepositoryImpl) CreateMessage(ctx context.Context, msg *model.Message) error {
	q := `
		INSERT INTO messages (
			id, sender_id, receiver_id, body, is_group, created_at, modified_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, q, msg.ID, msg.SenderID, msg.ReceiverID, msg.Body, msg.IsGroup, msg.CreatedAt, msg.ModifiedAt)
	return err
}

func (r *MessageRepositoryImpl) GetMessagesByReceiver(ctx context.Context, receiverID string, limit, offset int) (model.Messages, error) {
	query := `
		SELECT id, sender_id, receiver_id, body, is_group, created_at, modified_at
		FROM messages
		WHERE receiver_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, receiverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages model.Messages
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Body, &msg.IsGroup, &msg.CreatedAt, &msg.ModifiedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	return messages, rows.Err()
}

func (r *MessageRepositoryImpl) GetMessagesBetweenUsers(ctx context.Context, senderID, receiverID string, limit, offset int) (model.Messages, error) {
	query := `
		SELECT id, sender_id, receiver_id, body, is_group, created_at, modified_at
		FROM messages
		WHERE (sender_id = $1 AND receiver_id = $2) OR (sender_id = $2 AND receiver_id = $1)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, senderID, receiverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages model.Messages
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Body, &msg.IsGroup, &msg.CreatedAt, &msg.ModifiedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	return messages, rows.Err()
}
