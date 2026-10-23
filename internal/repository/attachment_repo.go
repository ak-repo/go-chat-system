package repository

import (
	"context"
	"errors"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentRepository struct{ db *pgxpool.Pool }

func NewAttachmentRepository(db *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

const attachmentColumns = `id,owner_id,conversation_id,filename,declared_mime,declared_size,mime_type,size_bytes,width,height,staging_key,full_key,thumbnail_key,state,error,upload_expires_at,attempts`

func scanAttachment(row pgx.Row) (*model.Attachment, error) {
	a := new(model.Attachment)
	err := row.Scan(&a.ID, &a.OwnerID, &a.ConversationID, &a.Filename, &a.DeclaredMIME, &a.DeclaredSize, &a.MIMEType, &a.Size, &a.Width, &a.Height, &a.StagingKey, &a.FullKey, &a.ThumbnailKey, &a.State, &a.Error, &a.UploadExpiresAt, &a.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return a, err
}
func (r *AttachmentRepository) Create(ctx context.Context, a *model.Attachment) error {
	_, err := r.db.Exec(ctx, `INSERT INTO attachments(id,owner_id,conversation_id,filename,declared_mime,declared_size,staging_key,full_key,thumbnail_key,upload_expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, a.ID, a.OwnerID, a.ConversationID, a.Filename, a.DeclaredMIME, a.DeclaredSize, a.StagingKey, a.FullKey, a.ThumbnailKey, a.UploadExpiresAt)
	return err
}
func (r *AttachmentRepository) Get(ctx context.Context, id string) (*model.Attachment, error) {
	return scanAttachment(r.db.QueryRow(ctx, `SELECT `+attachmentColumns+` FROM attachments WHERE id=$1`, id))
}
func (r *AttachmentRepository) Complete(ctx context.Context, id, owner string) error {
	result, err := r.db.Exec(ctx, `UPDATE attachments SET state='queued',updated_at=NOW() WHERE id=$1 AND owner_id=$2 AND state='pending' AND created_at>NOW()-INTERVAL '24 hours'`, id, owner)
	if err != nil {
		return err
	}
	if result.RowsAffected() > 0 {
		return nil
	}
	a, err := r.Get(ctx, id)
	if err != nil {
		return err
	}
	if a.OwnerID != owner {
		return errs.ErrNotFound
	}
	if a.State == "queued" || a.State == "processing" || a.State == "ready" {
		return nil
	}
	return errs.ErrConflict
}
func (r *AttachmentRepository) Cancel(ctx context.Context, id, owner string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	a, err := scanAttachment(tx.QueryRow(ctx, `SELECT `+attachmentColumns+` FROM attachments WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, owner))
	if err != nil {
		return err
	}
	if a.State == "deleting" || a.State == "deleted" {
		return nil
	}
	var linked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM message_attachments WHERE attachment_id=$1)`, id).Scan(&linked); err != nil {
		return err
	}
	if linked {
		return errs.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE attachments SET state='deleting',lease_until=NULL,updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Processing leases survive server restarts; attempts fence stale workers.
func (r *AttachmentRepository) Claim(ctx context.Context) (*model.Attachment, error) {
	// Retire exhausted leases before claiming more work, including repeated crashes.
	if _, err := r.db.Exec(ctx, `UPDATE attachments SET state='failed',error='Image processing attempts exhausted',lease_until=NULL,updated_at=NOW() WHERE attempts>=3 AND (state='queued' OR (state='processing' AND lease_until<NOW()))`); err != nil {
		return nil, err
	}
	return scanAttachment(r.db.QueryRow(ctx, `UPDATE attachments SET state='processing',attempts=attempts+1,lease_until=NOW()+INTERVAL '5 minutes',updated_at=NOW()
 WHERE id=(SELECT id FROM attachments WHERE attempts<3 AND (state='queued' OR (state='processing' AND lease_until<NOW())) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+attachmentColumns))
}
func (r *AttachmentRepository) Finish(ctx context.Context, a *model.Attachment, state, reason string) error {
	result, err := r.db.Exec(ctx, `UPDATE attachments SET state=$3,error=$4,mime_type=$5,size_bytes=$6,width=$7,height=$8,lease_until=NULL,updated_at=NOW() WHERE id=$1 AND state='processing' AND attempts=$2`, a.ID, a.Attempts, state, reason, a.MIMEType, a.Size, a.Width, a.Height)
	if err == nil && result.RowsAffected() != 1 {
		return errs.ErrConflict
	}
	return err
}

// Publish holds the same row lock used by cancellation and cleanup while the
// service writes bounded objects. SQL and transaction ownership stay here.
func (r *AttachmentRepository) Publish(ctx context.Context, a *model.Attachment, write func() error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	current, err := scanAttachment(tx.QueryRow(ctx, `SELECT `+attachmentColumns+` FROM attachments WHERE id=$1 FOR UPDATE`, a.ID))
	if err != nil {
		return err
	}
	if current.State != "processing" || current.Attempts != a.Attempts {
		return errs.ErrConflict
	}
	if err = write(); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE attachments SET state='ready',error='',mime_type=$2,size_bytes=$3,width=$4,height=$5,lease_until=NULL,updated_at=NOW() WHERE id=$1`, a.ID, a.MIMEType, a.Size, a.Width, a.Height)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Locking the attachment serializes cleanup with message linking and forwarding.
func (r *AttachmentRepository) ClaimCleanup(ctx context.Context) (*model.Attachment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	a, err := scanAttachment(tx.QueryRow(ctx, `SELECT `+attachmentColumns+` FROM attachments x WHERE x.upload_expires_at<NOW()
 AND ((x.state='deleting' AND (x.lease_until IS NULL OR x.lease_until<NOW()))
 OR (x.state IN ('pending','failed','ready') AND
 (x.created_at<NOW()-INTERVAL '24 hours' OR EXISTS(SELECT 1 FROM message_attachments ma WHERE ma.attachment_id=x.id))
 AND NOT EXISTS(SELECT 1 FROM message_attachments ma JOIN messages m ON m.id=ma.message_id WHERE ma.attachment_id=x.id AND m.deleted_at IS NULL)))
 ORDER BY x.updated_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err != nil {
		return nil, err
	}
	// A separate statement sees links committed while the candidate scan ran.
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM message_attachments ma JOIN messages m ON m.id=ma.message_id WHERE ma.attachment_id=$1 AND m.deleted_at IS NULL)`, a.ID).Scan(&live); err != nil {
		return nil, err
	}
	if live {
		return nil, errs.ErrNotFound
	}
	_, err = tx.Exec(ctx, `UPDATE attachments SET state='deleting',lease_until=NOW()+INTERVAL '5 minutes',updated_at=NOW() WHERE id=$1`, a.ID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	a.State = "deleting"
	return a, nil
}
func (r *AttachmentRepository) Deleted(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE attachments SET state='deleted',lease_until=NULL,updated_at=NOW() WHERE id=$1 AND state='deleting'`, id)
	return err
}
func (r *AttachmentRepository) ExpiredStaging(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT staging_key FROM attachments WHERE state='ready' AND NOT staging_cleaned AND upload_expires_at<NOW() LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err = rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
func (r *AttachmentRepository) StagingCleaned(ctx context.Context, key string) error {
	_, err := r.db.Exec(ctx, `UPDATE attachments SET staging_cleaned=true WHERE staging_key=$1`, key)
	return err
}
func (r *AttachmentRepository) ForMessage(ctx context.Context, id string) ([]model.Attachment, error) {
	return attachmentsForMessage(ctx, r.db, id)
}

type attachmentQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func attachmentsForMessage(ctx context.Context, q attachmentQuerier, id string) ([]model.Attachment, error) {
	rows, err := q.Query(ctx, `SELECT a.id,a.filename,a.mime_type,a.size_bytes,a.width,a.height,a.state FROM attachments a JOIN message_attachments ma ON ma.attachment_id=a.id WHERE ma.message_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Attachment{}
	for rows.Next() {
		var a model.Attachment
		if err = rows.Scan(&a.ID, &a.Filename, &a.MIMEType, &a.Size, &a.Width, &a.Height, &a.State); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// linkImage runs inside the message transaction, after conversation authorization.
func linkImage(ctx context.Context, tx pgx.Tx, m *model.Message) error {
	if len(m.AttachmentIDs) == 0 {
		return nil
	}
	if len(m.AttachmentIDs) != 1 {
		return errs.ErrValidation
	}
	a, err := scanAttachment(tx.QueryRow(ctx, `SELECT `+attachmentColumns+` FROM attachments WHERE id=$1 FOR UPDATE`, m.AttachmentIDs[0]))
	if err != nil {
		return err
	}
	if a.State != "ready" {
		return errs.ErrConflict
	}
	if m.ForwardedFromMessageID != "" {
		// Source visibility is rechecked in the transaction, including join-time history.
		var visible bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM message_attachments ma JOIN messages src ON src.id=ma.message_id JOIN conversations c ON c.id=src.conversation_id JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$3 AND cm.left_at IS NULL WHERE ma.message_id=$1 AND ma.attachment_id=$2 AND src.deleted_at IS NULL AND (c.kind='direct' OR src.created_at>=cm.joined_at))`, m.ForwardedFromMessageID, a.ID, m.SenderID).Scan(&visible)
		if err != nil {
			return err
		}
		if !visible {
			return errs.ErrForbidden
		}
	} else {
		if a.OwnerID != m.SenderID || a.ConversationID != m.ConversationID {
			return errs.ErrForbidden
		}
		var used bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM message_attachments WHERE attachment_id=$1)`, a.ID).Scan(&used); err != nil {
			return err
		}
		if used {
			return errs.ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO message_attachments(message_id,attachment_id) VALUES($1,$2)`, m.ID, a.ID)
	if err == nil {
		m.Attachments = []model.Attachment{*a}
		m.MessageType = "image"
	}
	return err
}
