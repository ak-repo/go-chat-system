package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepository interface {
	List(context.Context, string, int, int) ([]model.Notification, error)
	UnreadCount(context.Context, string) (int, error)
	MarkRead(context.Context, string, string, time.Time) (*model.Notification, error)
	MarkAllRead(context.Context, string, time.Time) ([]model.Notification, error)
	GetPreferences(context.Context, string) (model.NotificationPreferences, error)
	UpdatePreferences(context.Context, string, *bool, *bool, *bool) (model.NotificationPreferences, error)
}

type NotificationRepositoryImpl struct{ db *pgxpool.Pool }

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepositoryImpl {
	return &NotificationRepositoryImpl{db: db}
}

func scanNotification(row pgx.Row) (*model.Notification, error) {
	var n model.Notification
	var payload []byte
	if err := row.Scan(&n.ID, &n.RecipientID, &n.ActorID, &n.Type, &n.ConversationID, &n.MessageID, &payload, &n.CreatedAt, &n.ReadAt); err != nil {
		return nil, err
	}
	n.Payload = map[string]any{}
	if err := json.Unmarshal(payload, &n.Payload); err != nil {
		return nil, err
	}
	return &n, nil
}

const notificationColumns = `id,recipient_id,actor_id,type,conversation_id,message_id,payload,created_at,read_at`

func (r *NotificationRepositoryImpl) List(ctx context.Context, uid string, limit, offset int) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx, `SELECT `+notificationColumns+` FROM notifications WHERE recipient_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, uid, limit, offset)
	if err != nil {
		return nil, errs.Wrap("repository.Notification.List", err)
	}
	defer rows.Close()
	out := []model.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, errs.Wrap("repository.Notification.List", rows.Err())
}

func (r *NotificationRepositoryImpl) UnreadCount(ctx context.Context, uid string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_id=$1 AND read_at IS NULL`, uid).Scan(&count)
	return count, errs.Wrap("repository.Notification.Unread", err)
}

func (r *NotificationRepositoryImpl) MarkRead(ctx context.Context, id, uid string, at time.Time) (*model.Notification, error) {
	n, err := scanNotification(r.db.QueryRow(ctx, `UPDATE notifications SET read_at=COALESCE(read_at,$3) WHERE id=$1 AND recipient_id=$2 RETURNING `+notificationColumns, id, uid, at))
	if err == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return n, errs.Wrap("repository.Notification.Read", err)
}

func (r *NotificationRepositoryImpl) MarkAllRead(ctx context.Context, uid string, at time.Time) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx, `UPDATE notifications SET read_at=$2 WHERE recipient_id=$1 AND read_at IS NULL RETURNING `+notificationColumns, uid, at)
	if err != nil {
		return nil, errs.Wrap("repository.Notification.ReadAll", err)
	}
	defer rows.Close()
	out := []model.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, errs.Wrap("repository.Notification.ReadAll", rows.Err())
}

func (r *NotificationRepositoryImpl) GetPreferences(ctx context.Context, uid string) (model.NotificationPreferences, error) {
	var p model.NotificationPreferences
	err := r.db.QueryRow(ctx, `SELECT COALESCE(message_enabled,TRUE),COALESCE(reply_enabled,TRUE),COALESCE(mention_enabled,TRUE) FROM (SELECT $1::uuid AS user_id) u LEFT JOIN notification_preferences p USING(user_id)`, uid).Scan(&p.MessageEnabled, &p.ReplyEnabled, &p.MentionEnabled)
	return p, errs.Wrap("repository.Notification.Preferences", err)
}

func (r *NotificationRepositoryImpl) UpdatePreferences(ctx context.Context, uid string, message, reply, mention *bool) (model.NotificationPreferences, error) {
	var p model.NotificationPreferences
	err := r.db.QueryRow(ctx, `INSERT INTO notification_preferences(user_id,message_enabled,reply_enabled,mention_enabled) VALUES($1,COALESCE($2,TRUE),COALESCE($3,TRUE),COALESCE($4,TRUE)) ON CONFLICT(user_id) DO UPDATE SET message_enabled=COALESCE($2,notification_preferences.message_enabled),reply_enabled=COALESCE($3,notification_preferences.reply_enabled),mention_enabled=COALESCE($4,notification_preferences.mention_enabled),modified_at=NOW() RETURNING message_enabled,reply_enabled,mention_enabled`, uid, message, reply, mention).Scan(&p.MessageEnabled, &p.ReplyEnabled, &p.MentionEnabled)
	return p, errs.Wrap("repository.Notification.UpdatePreferences", err)
}
