package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepository interface {
	GetOrCreateDMChat(ctx context.Context, userID1, userID2 string) (*model.Chat, error)
	GetUserChats(ctx context.Context, userID string) ([]*model.ChatWithMembers, error)
	GetChatByID(ctx context.Context, chatID string) (*model.Chat, error)
	IsMember(ctx context.Context, chatID, userID string) (bool, error)
}

type ChatRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewChatRepositoryImpl(db *pgxpool.Pool) *ChatRepositoryImpl {
	return &ChatRepositoryImpl{db: db}
}

func (r *ChatRepositoryImpl) GetOrCreateDMChat(ctx context.Context, userID1, userID2 string) (*model.Chat, error) {
	// Find existing DM chat with exactly these two members
	q := `
		SELECT c.id, c.type, c.created_at
		FROM chats c
		INNER JOIN chat_members m ON c.id = m.chat_id
		WHERE c.type = 'dm' AND m.user_id IN ($1, $2)
		GROUP BY c.id, c.type, c.created_at
		HAVING COUNT(DISTINCT m.user_id) = 2
		LIMIT 1
	`
	var chat model.Chat
	err := r.db.QueryRow(ctx, q, userID1, userID2).Scan(&chat.ID, &chat.Type, &chat.CreatedAt)
	if err == nil {
		return &chat, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	// Create new DM chat
	chat.ID = uuid.New().String()
	chat.Type = "dm"
	_, err = r.db.Exec(ctx, `INSERT INTO chats (id, type) VALUES ($1, $2)`, chat.ID, chat.Type)
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO chat_members (chat_id, user_id) VALUES ($1, $2), ($1, $3)`,
		chat.ID, userID1, userID2)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `SELECT created_at FROM chats WHERE id = $1`, chat.ID).Scan(&chat.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *ChatRepositoryImpl) GetUserChats(ctx context.Context, userID string) ([]*model.ChatWithMembers, error) {
	q := `
		SELECT c.id, c.type, c.created_at
		FROM chats c
		INNER JOIN chat_members m ON c.id = m.chat_id
		WHERE m.user_id = $1
		ORDER BY c.created_at DESC
	`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*model.ChatWithMembers
	for rows.Next() {
		var c model.ChatWithMembers
		if err := rows.Scan(&c.ID, &c.Type, &c.CreatedAt); err != nil {
			return nil, err
		}
		members, err := r.getChatMembers(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		c.MemberIDs = members
		chats = append(chats, &c)
	}
	return chats, rows.Err()
}

func (r *ChatRepositoryImpl) getChatMembers(ctx context.Context, chatID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id FROM chat_members WHERE chat_id = $1`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ChatRepositoryImpl) GetChatByID(ctx context.Context, chatID string) (*model.Chat, error) {
	var c model.Chat
	err := r.db.QueryRow(ctx, `SELECT id, type, created_at FROM chats WHERE id = $1`, chatID).
		Scan(&c.ID, &c.Type, &c.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *ChatRepositoryImpl) IsMember(ctx context.Context, chatID, userID string) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2`, chatID, userID).Scan(&n)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
