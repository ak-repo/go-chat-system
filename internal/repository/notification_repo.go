package repository

import (
	"context"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepository interface {
	CreateNotification(ctx context.Context, n *model.Notification) error
	GetNotificationsForUser(ctx context.Context, userID string, limit, offset int) (model.Notifications, error)
	GetUnreadCount(ctx context.Context, userID string) (int, error)
	MarkAsRead(ctx context.Context, notificationID, userID string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	DeleteNotification(ctx context.Context, notificationID, userID string) error
	GetNotificationByID(ctx context.Context, notificationID string) (*model.Notification, error)
}

type NotificationRepositoryImpl struct {
	db *pgxpool.Pool
}

func NewNotificationRepositoryImpl(db *pgxpool.Pool) *NotificationRepositoryImpl {
	return &NotificationRepositoryImpl{db: db}
}

func (r *NotificationRepositoryImpl) CreateNotification(ctx context.Context, n *model.Notification) error {
	q := `
		INSERT INTO notifications (
			id, user_id, type, title, body, sender_id, reference_id, is_read, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, q, n.ID, n.UserID, n.Type, n.Title, n.Body, n.SenderID, n.ReferenceID, n.IsRead, n.CreatedAt)
	return err
}

func (r *NotificationRepositoryImpl) GetNotificationsForUser(ctx context.Context, userID string, limit, offset int) (model.Notifications, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, user_id, type, title, body, sender_id, reference_id, is_read, created_at
		FROM notifications
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications model.Notifications
	for rows.Next() {
		var n model.NotificationDTO
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.SenderID, &n.ReferenceID, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}
	return notifications, rows.Err()
}

func (r *NotificationRepositoryImpl) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1 AND is_read = false AND deleted_at IS NULL
	`, userID).Scan(&count)
	return count, err
}

func (r *NotificationRepositoryImpl) MarkAsRead(ctx context.Context, notificationID, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET is_read = true
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, notificationID, userID)
	return err
}

func (r *NotificationRepositoryImpl) MarkAllAsRead(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET is_read = true
		WHERE user_id = $1 AND is_read = false AND deleted_at IS NULL
	`, userID)
	return err
}

func (r *NotificationRepositoryImpl) DeleteNotification(ctx context.Context, notificationID, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET deleted_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, notificationID, userID)
	return err
}

func (r *NotificationRepositoryImpl) GetNotificationByID(ctx context.Context, notificationID string) (*model.Notification, error) {
	var n model.Notification
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, type, title, body, sender_id, reference_id, is_read, created_at, deleted_at
		FROM notifications
		WHERE id = $1
	`, notificationID).Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.SenderID, &n.ReferenceID, &n.IsRead, &n.CreatedAt, &n.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &n, nil
}