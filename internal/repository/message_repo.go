package repository

import (
	"context"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository interface {
	CreateMessage(ctx context.Context, msg *model.Message) error
	GetMessagesByReceiver(ctx context.Context, receiverID string, limit, offset int) (model.Messages, error)
	GetMessagesBetweenUsers(ctx context.Context, senderID, receiverID string, limit, offset int) (model.Messages, error)
}

type MessageMutationRepository interface {
	GetMessage(context.Context, string) (*model.Message, error)
	GetByClientMessageID(context.Context, string, string, string) (*model.Message, error)
	EditMessage(context.Context, string, string, string) error
	DeleteMessage(context.Context, string, string) error
	CreateReply(context.Context, *model.Message) error
	MarkDelivery(context.Context, string, string, string, time.Time) error
	MarkRead(context.Context, string, string, string, time.Time) error
	UnreadSummary(context.Context, string) (map[string]int, error)
}

type MessageReadAllRepository interface {
	MarkAllActiveRead(context.Context, string, time.Time) ([]model.ReadReceipt, error)
}

type MessageRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewMessageRepositoryImpl(db *pgxpool.Pool) *MessageRepositoryImpl {
	return &MessageRepositoryImpl{db: db}
}

func (r *MessageRepositoryImpl) CreateMessage(ctx context.Context, msg *model.Message) error {
	q := `
		INSERT INTO messages (id, sender_id, receiver_id, body, is_group, created_at, modified_at, conversation_id, client_message_id, reply_to_message_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9,'')::uuid, NULLIF($10,'')::uuid)
	`
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Wrap("repository.MessageRepository.CreateMessage", err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, q, msg.ID, msg.SenderID, msg.ReceiverID, msg.Body, msg.IsGroup, msg.CreatedAt, msg.ModifiedAt, msg.ConversationID, msg.ClientMessageID, msg.ReplyToMessageID)
	if err != nil {
		return errs.Wrap("repository.MessageRepository.CreateMessage", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO message_deliveries(message_id,recipient_id,status,status_at) VALUES($1,$2,'sent',$3) ON CONFLICT DO NOTHING`, msg.ID, msg.ReceiverID, msg.CreatedAt)
	if err != nil {
		return errs.Wrap("repository.MessageRepository.CreateDelivery", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return errs.Wrap("repository.MessageRepository.CreateMessage", err)
	}
	msg.Status = "sent"
	return nil
}
func (r *MessageRepositoryImpl) GetMessage(ctx context.Context, id string) (*model.Message, error) {
	var m model.Message
	e := r.db.QueryRow(ctx, `SELECT m.id,m.sender_id,m.receiver_id,m.body,m.is_group,m.created_at,m.modified_at,m.conversation_id,COALESCE(m.client_message_id::text,''),m.edited_at,COALESCE(m.reply_to_message_id::text,''),COALESCE(d.status,'sent') FROM messages m LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=m.receiver_id WHERE m.id=$1 AND m.deleted_at IS NULL`, id).Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Body, &m.IsGroup, &m.CreatedAt, &m.ModifiedAt, &m.ConversationID, &m.ClientMessageID, &m.EditedAt, &m.ReplyToMessageID, &m.Status)
	if e == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return &m, errs.Wrap("repository.Message.Get", e)
}
func (r *MessageRepositoryImpl) GetByClientMessageID(ctx context.Context, cid, sender, conversation string) (*model.Message, error) {
	var m model.Message
	e := r.db.QueryRow(ctx, `SELECT m.id,m.sender_id,m.receiver_id,m.body,m.is_group,m.created_at,m.modified_at,m.conversation_id,m.client_message_id,m.edited_at,m.reply_to_message_id,COALESCE(d.status,'sent') FROM messages m LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=m.receiver_id WHERE m.client_message_id=$1 AND m.sender_id=$2 AND m.conversation_id=$3`, cid, sender, conversation).Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Body, &m.IsGroup, &m.CreatedAt, &m.ModifiedAt, &m.ConversationID, &m.ClientMessageID, &m.EditedAt, &m.ReplyToMessageID, &m.Status)
	if e == pgx.ErrNoRows {
		return nil, nil
	}
	return &m, errs.Wrap("repository.Message.GetByClientMessageID", e)
}

func (r *MessageRepositoryImpl) GetMessagesByReceiver(ctx context.Context, receiverID string, limit, offset int) (model.Messages, error) {
	query := `
		SELECT m.id, m.sender_id, m.receiver_id, m.body, m.is_group, m.created_at, m.modified_at, m.conversation_id, COALESCE(m.client_message_id::text,''), m.edited_at, COALESCE(m.reply_to_message_id::text,''), COALESCE(d.status,'sent')
		FROM messages m LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=m.receiver_id
		WHERE m.receiver_id = $1 AND m.deleted_at IS NULL
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, receiverID, limit, offset)
	if err != nil {
		return nil, errs.Wrap("repository.MessageRepository.GetMessagesByReceiver", err)
	}
	defer rows.Close()

	var messages model.Messages
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Body, &msg.IsGroup, &msg.CreatedAt, &msg.ModifiedAt, &msg.ConversationID, &msg.ClientMessageID, &msg.EditedAt, &msg.ReplyToMessageID, &msg.Status); err != nil {
			return nil, errs.Wrap("repository.MessageRepository.GetMessagesByReceiver", err)
		}
		messages = append(messages, &msg)
	}
	return messages, errs.Wrap("repository.MessageRepository.GetMessagesByReceiver", rows.Err())
}

func (r *MessageRepositoryImpl) GetMessagesBetweenUsers(ctx context.Context, senderID, receiverID string, limit, offset int) (model.Messages, error) {
	query := `
		SELECT m.id, m.sender_id, m.receiver_id, m.body, m.is_group, m.created_at, m.modified_at, m.conversation_id, COALESCE(m.client_message_id::text,''), m.edited_at, COALESCE(m.reply_to_message_id::text,''), COALESCE(d.status,'sent')
		FROM messages m LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=m.receiver_id
		WHERE ((m.sender_id = $1 AND m.receiver_id = $2) OR (m.sender_id = $2 AND m.receiver_id = $1)) AND m.deleted_at IS NULL
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, senderID, receiverID, limit, offset)
	if err != nil {
		return nil, errs.Wrap("repository.MessageRepository.GetMessagesBetweenUsers", err)
	}
	defer rows.Close()

	var messages model.Messages
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Body, &msg.IsGroup, &msg.CreatedAt, &msg.ModifiedAt, &msg.ConversationID, &msg.ClientMessageID, &msg.EditedAt, &msg.ReplyToMessageID, &msg.Status); err != nil {
			return nil, errs.Wrap("repository.MessageRepository.GetMessagesBetweenUsers", err)
		}
		messages = append(messages, &msg)
	}
	return messages, errs.Wrap("repository.MessageRepository.GetMessagesBetweenUsers", rows.Err())
}

func (r *MessageRepositoryImpl) EditMessage(ctx context.Context, id, senderID, body string) error {
	res, e := r.db.Exec(ctx, `UPDATE messages SET body=$3,edited_at=NOW(),modified_at=NOW() WHERE id=$1 AND sender_id=$2 AND deleted_at IS NULL`, id, senderID, body)
	if e == nil && res.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return errs.Wrap("repository.Message.Edit", e)
}
func (r *MessageRepositoryImpl) DeleteMessage(ctx context.Context, id, senderID string) error {
	res, e := r.db.Exec(ctx, `UPDATE messages SET deleted_at=NOW(),modified_at=NOW() WHERE id=$1 AND sender_id=$2 AND deleted_at IS NULL`, id, senderID)
	if e == nil && res.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return errs.Wrap("repository.Message.Delete", e)
}
func (r *MessageRepositoryImpl) CreateReply(ctx context.Context, m *model.Message) error {
	return r.CreateMessage(ctx, m)
}
func (r *MessageRepositoryImpl) MarkDelivery(ctx context.Context, messageID, recipient, status string, at time.Time) error {
	res, e := r.db.Exec(ctx, `INSERT INTO message_deliveries(message_id,recipient_id,status,status_at) SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM messages WHERE id=$1 AND receiver_id=$2 AND deleted_at IS NULL) ON CONFLICT(message_id,recipient_id) DO UPDATE SET status=EXCLUDED.status,status_at=EXCLUDED.status_at, failure_code=NULL WHERE CASE message_deliveries.status WHEN 'read' THEN 3 WHEN 'delivered' THEN 2 WHEN 'sent' THEN 1 ELSE 0 END < CASE EXCLUDED.status WHEN 'read' THEN 3 WHEN 'delivered' THEN 2 WHEN 'sent' THEN 1 ELSE 0 END`, messageID, recipient, status, at)
	if e == nil && res.RowsAffected() == 0 {
		var exists bool
		if qe := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1 AND receiver_id=$2 AND deleted_at IS NULL)`, messageID, recipient).Scan(&exists); qe != nil {
			return errs.Wrap("repository.Message.Delivery", qe)
		}
		if !exists {
			return errs.ErrNotFound
		}
	}
	return errs.Wrap("repository.Message.Delivery", e)
}
func (r *MessageRepositoryImpl) MarkRead(ctx context.Context, cid, uid, messageID string, at time.Time) error {
	tx, e := r.db.Begin(ctx)
	if e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	defer tx.Rollback(ctx)
	var created time.Time
	if e = tx.QueryRow(ctx, `SELECT created_at FROM messages WHERE id=$1 AND conversation_id=$2 AND receiver_id=$3 AND deleted_at IS NULL`, messageID, cid, uid).Scan(&created); e == pgx.ErrNoRows {
		return errs.ErrNotFound
	} else if e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	if _, e = tx.Exec(ctx, `UPDATE message_deliveries d SET status='read',status_at=$4,failure_code=NULL FROM messages m WHERE d.message_id=m.id AND d.recipient_id=$2 AND m.conversation_id=$1 AND m.receiver_id=$2 AND (m.created_at,m.id) <= ($3,$5::uuid) AND d.status <> 'read'`, cid, uid, created, at, messageID); e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO conversation_read_state(conversation_id,user_id,last_read_message_id,last_read_at) VALUES($1,$2,$3,$4) ON CONFLICT(conversation_id,user_id) DO UPDATE SET last_read_message_id=EXCLUDED.last_read_message_id,last_read_at=EXCLUDED.last_read_at WHERE conversation_read_state.last_read_message_id IS NULL OR (SELECT (created_at,id) FROM messages WHERE id=EXCLUDED.last_read_message_id) >= (SELECT (created_at,id) FROM messages WHERE id=conversation_read_state.last_read_message_id)`, cid, uid, messageID, at); e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	return errs.Wrap("repository.Message.Read", tx.Commit(ctx))
}
func (r *MessageRepositoryImpl) UnreadSummary(ctx context.Context, uid string) (map[string]int, error) {
	rows, e := r.db.Query(ctx, `SELECT m.conversation_id,COUNT(*) FROM message_deliveries d JOIN messages m ON m.id=d.message_id JOIN users sender ON sender.id=m.sender_id AND sender.deleted_at IS NULL WHERE d.recipient_id=$1 AND m.sender_id<>$1 AND d.status <> 'read' AND m.deleted_at IS NULL GROUP BY m.conversation_id`, uid)
	if e != nil {
		return nil, errs.Wrap("repository.Message.Unread", e)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if e = rows.Scan(&id, &n); e != nil {
			return nil, e
		}
		out[id] = n
	}
	return out, errs.Wrap("repository.Message.Unread", rows.Err())
}

func (r *MessageRepositoryImpl) MarkAllActiveRead(ctx context.Context, uid string, at time.Time) ([]model.ReadReceipt, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, errs.Wrap("repository.Message.MarkAllRead", err)
	}
	defer tx.Rollback(ctx)
	const latest = `
		WITH latest AS (
			SELECT m.conversation_id, (array_agg(m.id ORDER BY m.created_at DESC,m.id DESC))[1] AS message_id,
			       max(m.created_at) AS created_at
			FROM messages m
			JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$1 AND cm.left_at IS NULL
			LEFT JOIN conversation_user_state s ON s.conversation_id=m.conversation_id AND s.user_id=$1
			WHERE m.receiver_id=$1 AND m.deleted_at IS NULL
			  AND s.archived_at IS NULL AND s.hidden_at IS NULL
			GROUP BY m.conversation_id
		)
	`
	rows, err := tx.Query(ctx, latest+`SELECT l.conversation_id,l.message_id,m.sender_id FROM latest l JOIN messages m ON m.id=l.message_id`, uid)
	if err != nil {
		return nil, errs.Wrap("repository.Message.MarkAllRead", err)
	}
	var receipts []model.ReadReceipt
	for rows.Next() {
		var receipt model.ReadReceipt
		if err = rows.Scan(&receipt.ConversationID, &receipt.MessageID, &receipt.SenderID); err != nil {
			rows.Close()
			return nil, errs.Wrap("repository.Message.MarkAllRead", err)
		}
		receipts = append(receipts, receipt)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, errs.Wrap("repository.Message.MarkAllRead", err)
	}
	rows.Close()
	if _, err = tx.Exec(ctx, latest+`UPDATE message_deliveries d SET status='read',status_at=$2,failure_code=NULL FROM messages m,latest l WHERE d.message_id=m.id AND d.recipient_id=$1 AND m.conversation_id=l.conversation_id AND (m.created_at,m.id)<=(l.created_at,l.message_id) AND d.status <> 'read'`, uid, at); err != nil {
		return nil, errs.Wrap("repository.Message.MarkAllRead", err)
	}
	if _, err = tx.Exec(ctx, latest+`INSERT INTO conversation_read_state(conversation_id,user_id,last_read_message_id,last_read_at) SELECT conversation_id,$1,message_id,$2 FROM latest ON CONFLICT(conversation_id,user_id) DO UPDATE SET last_read_message_id=EXCLUDED.last_read_message_id,last_read_at=EXCLUDED.last_read_at WHERE (SELECT created_at FROM messages WHERE id=conversation_read_state.last_read_message_id) IS NULL OR (SELECT (created_at,id) FROM messages WHERE id=EXCLUDED.last_read_message_id) >= (SELECT (created_at,id) FROM messages WHERE id=conversation_read_state.last_read_message_id)`, uid, at); err != nil {
		return nil, errs.Wrap("repository.Message.MarkAllRead", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errs.Wrap("repository.Message.MarkAllRead", err)
	}
	return receipts, nil
}
