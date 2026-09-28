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
type PresenceRepository interface {
	PresenceAudience(context.Context, string) ([]string, error)
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
	var userOne, userTwo, name, description, creator, avatar *string
	err := row.Scan(&c.ID, &c.Kind, &userOne, &userTwo, &c.CreatedAt, &c.ModifiedAt, &archivedAt, &pinnedAt, &c.MutedUntil, &c.Muted, &name, &description, &creator, &avatar, &c.MemberCount, &c.CurrentRole)
	if userOne != nil {
		c.UserOneID = *userOne
	}
	if userTwo != nil {
		c.UserTwoID = *userTwo
	}
	if name != nil {
		c.Name = *name
	}
	if description != nil {
		c.Description = *description
	}
	if creator != nil {
		c.CreatorID = *creator
	}
	if avatar != nil {
		c.AvatarURL = *avatar
	}
	if c.Kind == "group" {
		c.Capabilities = groupCapabilities(c.CurrentRole)
	} else {
		// Group-only projections must not alter the direct conversation response.
		c.MemberCount = 0
		c.CurrentRole = ""
	}
	c.Archived = archivedAt != nil
	c.Pinned = pinnedAt != nil
	return &c, err
}

const conversationSelect = `c.id,c.kind,c.user_one_id,c.user_two_id,c.created_at,c.modified_at,s.archived_at,s.pinned_at,s.muted_until,COALESCE(s.muted_until>NOW(),FALSE),c.name,c.description,c.creator_id,c.avatar_url,(SELECT count(*) FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.left_at IS NULL),m.role`

func groupCapabilities(role string) *model.GroupCapabilities {
	privileged := role == model.GroupRoleOwner || role == model.GroupRoleAdmin
	return &model.GroupCapabilities{EditGroup: privileged, AddMembers: privileged, RemoveMembers: privileged, ManageRoles: role == model.GroupRoleOwner, ManageInvites: privileged}
}
func (r *ConversationRepositoryImpl) CreateOrGetDirect(ctx context.Context, a, b string) (*model.Conversation, error) {
	if a == b {
		return nil, errs.ErrSelfAction
	}
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

	var conversationID string
	err = tx.QueryRow(ctx, `SELECT id FROM conversations WHERE user_one_id=LEAST($1::uuid,$2::uuid) AND user_two_id=GREATEST($1::uuid,$2::uuid) AND kind='direct'`, a, b).Scan(&conversationID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.ErrNotFound
		}
		return nil, errs.Wrap("repository.Conversation.GetOrCreate", err)
	}

	_, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2::uuid),($1,$3::uuid) ON CONFLICT DO NOTHING`, conversationID, a, b)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.Member", err)
	}
	// Explicitly opening a direct conversation restores it only for the acting user.
	if _, err = tx.Exec(ctx, `UPDATE conversation_user_state SET hidden_at=NULL,modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, conversationID, a); err != nil {
		return nil, errs.Wrap("repository.Conversation.Unhide", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errs.Wrap("repository.Conversation.Commit", err)
	}
	return r.Get(ctx, conversationID, a)
}
func (r *ConversationRepositoryImpl) Get(ctx context.Context, id, uid string) (*model.Conversation, error) {
	c, e := scanConversation(r.db.QueryRow(ctx, `SELECT `+conversationSelect+` FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id AND m.user_id=$2 AND m.left_at IS NULL LEFT JOIN conversation_user_state s ON s.conversation_id=c.id AND s.user_id=$2 WHERE c.id=$1 AND COALESCE(s.hidden_at IS NULL,TRUE) AND (c.kind='group' OR (EXISTS(SELECT 1 FROM users WHERE id=c.user_one_id AND deleted_at IS NULL) AND EXISTS(SELECT 1 FROM users WHERE id=c.user_two_id AND deleted_at IS NULL)))`, id, uid))
	if e == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return c, errs.Wrap("repository.Conversation.Get", e)
}
func (r *ConversationRepositoryImpl) List(ctx context.Context, uid string, limit, offset int) ([]*model.Conversation, error) {
	return r.ListWithPreferences(ctx, uid, limit, offset, false)
}
func (r *ConversationRepositoryImpl) ListWithPreferences(ctx context.Context, uid string, limit, offset int, archived bool) ([]*model.Conversation, error) {
	rows, e := r.db.Query(ctx, `SELECT `+conversationSelect+` FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id LEFT JOIN conversation_user_state s ON s.conversation_id=c.id AND s.user_id=$1 WHERE m.user_id=$1 AND m.left_at IS NULL AND COALESCE(s.hidden_at IS NULL,TRUE) AND (COALESCE(s.archived_at IS NOT NULL,FALSE)=$4) AND (c.kind='group' OR (EXISTS(SELECT 1 FROM users WHERE id=c.user_one_id AND deleted_at IS NULL) AND EXISTS(SELECT 1 FROM users WHERE id=c.user_two_id AND deleted_at IS NULL))) ORDER BY (s.pinned_at IS NOT NULL) DESC,s.pinned_at DESC,c.modified_at DESC,c.id LIMIT $2 OFFSET $3`, uid, limit, offset, archived)
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

// PresenceAudience returns active users who may observe uid through either an
// active friendship or a shared active conversation. A block in either
// direction suppresses presence regardless of the relationship source.
func (r *ConversationRepositoryImpl) PresenceAudience(ctx context.Context, uid string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		WITH candidates AS (
			SELECT friend_id AS user_id FROM friends
			WHERE user_id=$1 AND deleted_at IS NULL
			UNION
			SELECT other.user_id
			FROM conversation_members mine
			JOIN conversation_members other ON other.conversation_id=mine.conversation_id
				AND other.user_id<>mine.user_id AND other.left_at IS NULL
			WHERE mine.user_id=$1 AND mine.left_at IS NULL
		)
		SELECT candidates.user_id
		FROM candidates
		JOIN users actor ON actor.id=$1 AND actor.deleted_at IS NULL
		JOIN users observer ON observer.id=candidates.user_id AND observer.deleted_at IS NULL
		WHERE NOT EXISTS (
			SELECT 1 FROM blocks b
			WHERE b.deleted_at IS NULL AND ((b.blocker_id=$1 AND b.blocked_id=candidates.user_id)
			   OR (b.blocker_id=candidates.user_id AND b.blocked_id=$1))
		)
		ORDER BY candidates.user_id`, uid)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.PresenceAudience", err)
	}
	defer rows.Close()
	var audience []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errs.Wrap("repository.Conversation.PresenceAudience", err)
		}
		audience = append(audience, id)
	}
	return audience, errs.Wrap("repository.Conversation.PresenceAudience", rows.Err())
}
