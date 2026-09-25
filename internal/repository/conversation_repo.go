package repository

import (
	"context"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversationRepository interface {
	CreateOrGetDirect(context.Context, string, string) (*model.Conversation, error)
	Get(context.Context, string, string) (*model.Conversation, error)
	List(context.Context, string, int, int) ([]*model.Conversation, error)
	IsMember(context.Context, string, string) (bool, error)
}
type ConversationPreferencesRepository interface {
	ListWithPreferences(context.Context, string, int, int, bool) ([]*model.Conversation, error)
	UpdatePreferences(context.Context, string, string, *bool, *bool, *time.Time) error
	Hide(context.Context, string, string) error
}
type ConversationRepositoryImpl struct{ db *pgxpool.Pool }

func NewConversationRepository(db *pgxpool.Pool) *ConversationRepositoryImpl {
	return &ConversationRepositoryImpl{db: db}
}
func scanConversation(row pgx.Row) (*model.Conversation, error) {
	var c model.Conversation
	var archivedAt, pinnedAt *time.Time
	err := row.Scan(&c.ID, &c.Kind, &c.UserOneID, &c.UserTwoID, &c.CreatedAt, &c.ModifiedAt, &archivedAt, &pinnedAt, &c.MutedUntil, &c.Muted)
	c.Archived = archivedAt != nil
	c.Pinned = pinnedAt != nil
	return &c, err
}
func (r *ConversationRepositoryImpl) CreateOrGetDirect(ctx context.Context, a, b string) (*model.Conversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.Begin", err)
	}
	defer tx.Rollback(ctx)

	id := uuid.NewString()
	_, err = tx.Exec(ctx, `INSERT INTO conversations(id,kind,user_one_id,user_two_id) VALUES($1,'direct',LEAST($2::uuid,$3::uuid),GREATEST($2::uuid,$3::uuid)) ON CONFLICT DO NOTHING`, id, a, b)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.Create", err)
	}

	c, err := scanConversation(tx.QueryRow(ctx, `SELECT c.id,c.kind,c.user_one_id,c.user_two_id,c.created_at,c.modified_at,s.archived_at,s.pinned_at,s.muted_until,COALESCE(s.muted_until>NOW(),FALSE) FROM conversations c LEFT JOIN conversation_user_state s ON s.conversation_id=c.id AND s.user_id=$1 WHERE c.user_one_id=LEAST($1::uuid,$2::uuid) AND c.user_two_id=GREATEST($1::uuid,$2::uuid) AND c.kind='direct'`, a, b))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrNotFound
		}
		return nil, errs.Wrap("repository.Conversation.GetOrCreate", err)
	}

	_, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2::uuid),($1,$3::uuid) ON CONFLICT DO NOTHING`, c.ID, a, b)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.Member", err)
	}
	// Explicitly opening a direct conversation restores it only for the acting user.
	if _, err = tx.Exec(ctx, `UPDATE conversation_user_state SET hidden_at=NULL,modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, c.ID, a); err != nil {
		return nil, errs.Wrap("repository.Conversation.Unhide", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errs.Wrap("repository.Conversation.Commit", err)
	}
	return c, nil
}
func (r *ConversationRepositoryImpl) Get(ctx context.Context, id, uid string) (*model.Conversation, error) {
	c, e := scanConversation(r.db.QueryRow(ctx, `SELECT c.id,c.kind,c.user_one_id,c.user_two_id,c.created_at,c.modified_at,s.archived_at,s.pinned_at,s.muted_until,COALESCE(s.muted_until>NOW(),FALSE) FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id AND m.user_id=$2 AND m.left_at IS NULL LEFT JOIN conversation_user_state s ON s.conversation_id=c.id AND s.user_id=$2 WHERE c.id=$1 AND COALESCE(s.hidden_at IS NULL,TRUE) AND EXISTS(SELECT 1 FROM users WHERE id=c.user_one_id AND deleted_at IS NULL) AND EXISTS(SELECT 1 FROM users WHERE id=c.user_two_id AND deleted_at IS NULL)`, id, uid))
	if e == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return c, errs.Wrap("repository.Conversation.Get", e)
}
func (r *ConversationRepositoryImpl) List(ctx context.Context, uid string, limit, offset int) ([]*model.Conversation, error) {
	return r.ListWithPreferences(ctx, uid, limit, offset, false)
}
func (r *ConversationRepositoryImpl) ListWithPreferences(ctx context.Context, uid string, limit, offset int, archived bool) ([]*model.Conversation, error) {
	rows, e := r.db.Query(ctx, `SELECT c.id,c.kind,c.user_one_id,c.user_two_id,c.created_at,c.modified_at,s.archived_at,s.pinned_at,s.muted_until,COALESCE(s.muted_until>NOW(),FALSE) FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id LEFT JOIN conversation_user_state s ON s.conversation_id=c.id AND s.user_id=$1 WHERE m.user_id=$1 AND m.left_at IS NULL AND COALESCE(s.hidden_at IS NULL,TRUE) AND (COALESCE(s.archived_at IS NOT NULL,FALSE)=$4) AND EXISTS(SELECT 1 FROM users WHERE id=c.user_one_id AND deleted_at IS NULL) AND EXISTS(SELECT 1 FROM users WHERE id=c.user_two_id AND deleted_at IS NULL) ORDER BY (s.pinned_at IS NOT NULL) DESC,s.pinned_at DESC,c.modified_at DESC,c.id LIMIT $2 OFFSET $3`, uid, limit, offset, archived)
	if e != nil {
		return nil, errs.Wrap("repository.Conversation.List", e)
	}
	defer rows.Close()
	var out []*model.Conversation
	for rows.Next() {
		c, e := scanConversation(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, errs.Wrap("repository.Conversation.List", rows.Err())
}
func (r *ConversationRepositoryImpl) UpdatePreferences(ctx context.Context, cid, uid string, archived, pinned *bool, mutedUntil *time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errs.Wrap("repository.Conversation.Preferences", err)
	}
	defer tx.Rollback(ctx)
	var member bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, cid, uid).Scan(&member); err != nil {
		return errs.Wrap("repository.Conversation.Preferences", err)
	}
	if !member {
		return errs.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO conversation_user_state(conversation_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, cid, uid); err != nil {
		return errs.Wrap("repository.Conversation.Preferences", err)
	}
	if archived != nil {
		if _, err = tx.Exec(ctx, `UPDATE conversation_user_state SET archived_at=CASE WHEN $3 THEN COALESCE(archived_at,NOW()) ELSE NULL END,modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, uid, *archived); err != nil {
			return errs.Wrap("repository.Conversation.Preferences", err)
		}
	}
	if pinned != nil {
		if _, err = tx.Exec(ctx, `UPDATE conversation_user_state SET pinned_at=CASE WHEN $3 THEN COALESCE(pinned_at,NOW()) ELSE NULL END,modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, uid, *pinned); err != nil {
			return errs.Wrap("repository.Conversation.Preferences", err)
		}
	}
	if mutedUntil != nil {
		if _, err = tx.Exec(ctx, `UPDATE conversation_user_state SET muted_until=$3,modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, uid, *mutedUntil); err != nil {
			return errs.Wrap("repository.Conversation.Preferences", err)
		}
	}
	return errs.Wrap("repository.Conversation.Preferences", tx.Commit(ctx))
}
func (r *ConversationRepositoryImpl) Hide(ctx context.Context, cid, uid string) error {
	res, err := r.db.Exec(ctx, `INSERT INTO conversation_user_state(conversation_id,user_id,hidden_at) SELECT $1,$2,NOW() WHERE EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL) ON CONFLICT(conversation_id,user_id) DO UPDATE SET hidden_at=NOW(),modified_at=NOW()`, cid, uid)
	if err == nil && res.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return errs.Wrap("repository.Conversation.Hide", err)
}
func (r *ConversationRepositoryImpl) IsMember(ctx context.Context, cid, uid string) (bool, error) {
	var ok bool
	e := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, cid, uid).Scan(&ok)
	return ok, errs.Wrap("repository.Conversation.Member", e)
}
