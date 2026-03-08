package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository interface {
	Create(ctx context.Context, msg *model.Message) error
	ListByChatID(ctx context.Context, chatID string, limit int, beforeID string) ([]*model.Message, error)
}

type MessageRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewMessageRepositoryImpl(db *pgxpool.Pool) *MessageRepositoryImpl {
	return &MessageRepositoryImpl{db: db}
}

func (r *MessageRepositoryImpl) Create(ctx context.Context, msg *model.Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	return r.db.QueryRow(ctx,
		`INSERT INTO messages (id, chat_id, sender_id, content, created_at) VALUES ($1, $2, $3, $4, NOW()) RETURNING created_at`,
		msg.ID, msg.ChatID, msg.SenderID, msg.Content).Scan(&msg.CreatedAt)
}

// ListByChatID returns messages in descending order (newest first). If beforeID is set, returns older messages (before that message's created_at).
func (r *MessageRepositoryImpl) ListByChatID(ctx context.Context, chatID string, limit int, beforeID string) ([]*model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if beforeID != "" {
		rows, err = r.db.Query(ctx,
			`SELECT id, chat_id, sender_id, content, created_at FROM messages WHERE chat_id = $1 AND created_at < (SELECT created_at FROM messages WHERE id = $2) ORDER BY created_at DESC LIMIT $3`,
			chatID, beforeID, limit)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, chat_id, sender_id, content, created_at FROM messages WHERE chat_id = $1 ORDER BY created_at DESC LIMIT $2`,
			chatID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &m)
	}
	return list, rows.Err()
}
