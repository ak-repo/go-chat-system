package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GroupConversationRepository interface {
	CreateGroup(context.Context, string, string, *string, []string) (*model.Conversation, error)
	UpdateGroup(context.Context, string, string, *string, *string) (*model.Conversation, error)
	ListGroupMembers(context.Context, string, string) ([]*model.GroupMember, error)
	AddGroupMembers(context.Context, string, string, []string) ([]*model.GroupMember, error)
	RemoveGroupMember(context.Context, string, string, string) error
	LeaveGroup(context.Context, string, string) error
	UpdateGroupMemberRole(context.Context, string, string, string, string) error
	CreateGroupInvite(context.Context, string, string, []byte, time.Time, *int) (*model.GroupInvite, error)
	ListGroupInvites(context.Context, string, string) ([]*model.GroupInvite, error)
	RevokeGroupInvite(context.Context, string, string, string) error
	AcceptGroupInvite(context.Context, string, []byte) (string, error)
}

func privilegedGroupRole(role string) bool {
	return role == model.GroupRoleOwner || role == model.GroupRoleAdmin
}

func (r *ConversationRepositoryImpl) CreateGroupInvite(ctx context.Context, cid, actor string, tokenHash []byte, expiresAt time.Time, maxUses *int) (*model.GroupInvite, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, ctx)
	role, err := lockGroupRole(ctx, tx, cid, actor)
	if err != nil {
		return nil, err
	}
	if !privilegedGroupRole(role) {
		return nil, errs.ErrForbidden
	}
	invite := &model.GroupInvite{ID: uuid.NewString(), ConversationID: cid, CreatorID: actor, ExpiresAt: expiresAt, MaxUses: maxUses}
	err = tx.QueryRow(ctx, `INSERT INTO group_invites(id,conversation_id,creator_id,token_hash,expires_at,max_uses) VALUES($1,$2,$3,$4,$5,$6) RETURNING created_at`, invite.ID, cid, actor, tokenHash, expiresAt, maxUses).Scan(&invite.CreatedAt)
	if err != nil {
		return nil, errs.Wrap("repository.GroupInvite.Create", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return invite, nil
}

func (r *ConversationRepositoryImpl) ListGroupInvites(ctx context.Context, cid, actor string) ([]*model.GroupInvite, error) {
	var role string
	err := r.db.QueryRow(ctx, `SELECT m.role FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE c.id=$1 AND c.kind='group' AND m.user_id=$2 AND m.left_at IS NULL`, cid, actor).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !privilegedGroupRole(role) {
		return nil, errs.ErrForbidden
	}
	rows, err := r.db.Query(ctx, `SELECT id,conversation_id,creator_id,expires_at,max_uses,use_count,revoked_at,created_at FROM group_invites WHERE conversation_id=$1 ORDER BY created_at DESC,id`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invites := []*model.GroupInvite{}
	for rows.Next() {
		i := &model.GroupInvite{}
		if err = rows.Scan(&i.ID, &i.ConversationID, &i.CreatorID, &i.ExpiresAt, &i.MaxUses, &i.UseCount, &i.RevokedAt, &i.CreatedAt); err != nil {
			return nil, err
		}
		invites = append(invites, i)
	}
	return invites, rows.Err()
}

func (r *ConversationRepositoryImpl) RevokeGroupInvite(ctx context.Context, cid, actor, inviteID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, ctx)
	role, err := lockGroupRole(ctx, tx, cid, actor)
	if err != nil {
		return err
	}
	if !privilegedGroupRole(role) {
		return errs.ErrForbidden
	}
	tag, err := tx.Exec(ctx, `UPDATE group_invites SET revoked_at=COALESCE(revoked_at,NOW()) WHERE id=$1 AND conversation_id=$2`, inviteID, cid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *ConversationRepositoryImpl) AcceptGroupInvite(ctx context.Context, actor string, tokenHash []byte) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer rollback(tx, ctx)
	var invite model.GroupInvite
	err = tx.QueryRow(ctx, `SELECT i.id,i.conversation_id,i.creator_id,i.expires_at,i.max_uses,i.use_count,i.revoked_at,i.created_at FROM group_invites i JOIN conversations c ON c.id=i.conversation_id AND c.kind='group' WHERE i.token_hash=$1 FOR UPDATE OF i,c`, tokenHash).Scan(&invite.ID, &invite.ConversationID, &invite.CreatorID, &invite.ExpiresAt, &invite.MaxUses, &invite.UseCount, &invite.RevokedAt, &invite.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errs.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	var userActive bool
	err = tx.QueryRow(ctx, `SELECT deleted_at IS NULL FROM users WHERE id=$1 FOR UPDATE`, actor).Scan(&userActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errs.ErrInactiveUser
	}
	if err != nil {
		return "", err
	}
	if !userActive {
		return "", errs.ErrInactiveUser
	}
	var leftAt *time.Time
	err = tx.QueryRow(ctx, `SELECT left_at FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 FOR UPDATE`, invite.ConversationID, actor).Scan(&leftAt)
	active := err == nil && leftAt == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if active {
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return invite.ConversationID, nil
	}
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(time.Now()) || (invite.MaxUses != nil && invite.UseCount >= *invite.MaxUses) {
		return "", errs.ErrConflict
	}
	var blocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM blocks WHERE deleted_at IS NULL AND ((blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1)))`, actor, invite.CreatorID).Scan(&blocked); err != nil {
		return "", err
	}
	if blocked {
		return "", errs.ErrForbidden
	}
	_, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) VALUES($1,$2,'member',$3) ON CONFLICT(conversation_id,user_id) DO UPDATE SET role='member',added_by=$3,joined_at=NOW(),left_at=NULL,modified_at=NOW()`, invite.ConversationID, actor, invite.CreatorID)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE group_invites SET use_count=use_count+1 WHERE id=$1`, invite.ID); err != nil {
		return "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return invite.ConversationID, nil
}

func HashGroupInviteToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func rollback(tx pgx.Tx, ctx context.Context) { _ = tx.Rollback(ctx) }

func (r *ConversationRepositoryImpl) CreateGroup(ctx context.Context, actor, name string, description *string, members []string) (*model.Conversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Wrap("repository.Group.Create", err)
	}
	defer rollback(tx, ctx)
	all := append([]string{actor}, members...)
	var eligible int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM users u WHERE u.id=ANY($1::uuid[]) AND u.deleted_at IS NULL AND (u.id=$2 OR (EXISTS(SELECT 1 FROM friends f WHERE f.user_id=$2 AND f.friend_id=u.id AND f.deleted_at IS NULL) AND NOT EXISTS(SELECT 1 FROM blocks b WHERE b.deleted_at IS NULL AND ((b.blocker_id=$2 AND b.blocked_id=u.id) OR (b.blocker_id=u.id AND b.blocked_id=$2)))))`, all, actor).Scan(&eligible)
	if err != nil {
		return nil, errs.Wrap("repository.Group.ValidateMembers", err)
	}
	if eligible != len(all) {
		return nil, errs.ErrForbidden
	}
	id := uuid.NewString()
	if _, err = tx.Exec(ctx, `INSERT INTO conversations(id,kind,name,description,creator_id) VALUES($1,'group',$2,$3,$4)`, id, name, description, actor); err != nil {
		return nil, errs.Wrap("repository.Group.Create", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,CASE WHEN x=$2 THEN 'owner' ELSE 'member' END,$2 FROM unnest($3::uuid[]) x`, id, actor, all); err != nil {
		return nil, errs.Wrap("repository.Group.Members", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errs.Wrap("repository.Group.Commit", err)
	}
	return r.Get(ctx, id, actor)
}

func lockGroupRole(ctx context.Context, tx pgx.Tx, cid, actor string) (string, error) {
	var role string
	err := tx.QueryRow(ctx, `SELECT m.role FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE c.id=$1 AND c.kind='group' AND m.user_id=$2 AND m.left_at IS NULL FOR UPDATE OF c,m`, cid, actor).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errs.ErrNotFound
	}
	return role, errs.Wrap("repository.Group.Authorize", err)
}

func (r *ConversationRepositoryImpl) UpdateGroup(ctx context.Context, cid, actor string, name, description *string) (*model.Conversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, ctx)
	role, err := lockGroupRole(ctx, tx, cid, actor)
	if err != nil {
		return nil, err
	}
	if role == model.GroupRoleMember {
		return nil, errs.ErrForbidden
	}
	_, err = tx.Exec(ctx, `UPDATE conversations SET name=COALESCE($2,name),description=CASE WHEN $3::boolean THEN $4 ELSE description END,modified_at=NOW() WHERE id=$1`, cid, name, description != nil, description)
	if err != nil {
		return nil, errs.Wrap("repository.Group.Update", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.Get(ctx, cid, actor)
}

func (r *ConversationRepositoryImpl) ListGroupMembers(ctx context.Context, cid, actor string) ([]*model.GroupMember, error) {
	var allowed bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE c.id=$1 AND c.kind='group' AND m.user_id=$2 AND m.left_at IS NULL)`, cid, actor).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errs.ErrNotFound
	}
	rows, err := r.db.Query(ctx, `SELECT m.user_id,u.username,m.role,m.joined_at,m.added_by,m.modified_at FROM conversation_members m JOIN users u ON u.id=m.user_id WHERE m.conversation_id=$1 AND m.left_at IS NULL ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,m.joined_at,m.user_id`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*model.GroupMember{}
	for rows.Next() {
		var m model.GroupMember
		var added *string
		if err = rows.Scan(&m.UserID, &m.Username, &m.Role, &m.JoinedAt, &added, &m.ModifiedAt); err != nil {
			return nil, err
		}
		if added != nil {
			m.AddedBy = *added
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (r *ConversationRepositoryImpl) AddGroupMembers(ctx context.Context, cid, actor string, members []string) ([]*model.GroupMember, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, ctx)
	role, err := lockGroupRole(ctx, tx, cid, actor)
	if err != nil {
		return nil, err
	}
	if role == model.GroupRoleMember {
		return nil, errs.ErrForbidden
	}
	var eligible int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM users u WHERE u.id=ANY($1::uuid[]) AND u.deleted_at IS NULL AND EXISTS(SELECT 1 FROM friends f WHERE f.user_id=$2 AND f.friend_id=u.id AND f.deleted_at IS NULL) AND NOT EXISTS(SELECT 1 FROM blocks b WHERE b.deleted_at IS NULL AND ((b.blocker_id=$2 AND b.blocked_id=u.id) OR (b.blocker_id=u.id AND b.blocked_id=$2)))`, members, actor).Scan(&eligible)
	if err != nil {
		return nil, err
	}
	if eligible != len(members) {
		return nil, errs.ErrForbidden
	}
	_, err = tx.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,'member',$2 FROM unnest($3::uuid[]) x ON CONFLICT(conversation_id,user_id) DO UPDATE SET role='member',added_by=$2,joined_at=NOW(),left_at=NULL,modified_at=NOW()`, cid, actor, members)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListGroupMembers(ctx, cid, actor)
}

func (r *ConversationRepositoryImpl) RemoveGroupMember(ctx context.Context, cid, actor, target string) error {
	return r.endMembership(ctx, cid, actor, target, false)
}
func (r *ConversationRepositoryImpl) LeaveGroup(ctx context.Context, cid, actor string) error {
	return r.endMembership(ctx, cid, actor, actor, true)
}
func (r *ConversationRepositoryImpl) endMembership(ctx context.Context, cid, actor, target string, leaving bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, ctx)
	actorRole, err := lockGroupRole(ctx, tx, cid, actor)
	if err != nil {
		return err
	}
	var targetRole string
	err = tx.QueryRow(ctx, `SELECT role FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL FOR UPDATE`, cid, target).Scan(&targetRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !leaving && (actor == target || actorRole == model.GroupRoleMember || (actorRole == model.GroupRoleAdmin && targetRole != model.GroupRoleMember)) {
		return errs.ErrForbidden
	}
	if targetRole == model.GroupRoleOwner {
		var owners int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM conversation_members WHERE conversation_id=$1 AND role='owner' AND left_at IS NULL`, cid).Scan(&owners); err != nil {
			return err
		}
		if owners == 1 {
			return errs.ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `UPDATE conversation_members SET left_at=NOW(),modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, target)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *ConversationRepositoryImpl) UpdateGroupMemberRole(ctx context.Context, cid, actor, target, newRole string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, ctx)
	role, err := lockGroupRole(ctx, tx, cid, actor)
	if err != nil {
		return err
	}
	if role != model.GroupRoleOwner {
		return errs.ErrForbidden
	}
	var old string
	err = tx.QueryRow(ctx, `SELECT role FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL FOR UPDATE`, cid, target).Scan(&old)
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrNotFound
	}
	if err != nil {
		return err
	}
	if old == model.GroupRoleOwner && newRole != model.GroupRoleOwner {
		var owners int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM conversation_members WHERE conversation_id=$1 AND role='owner' AND left_at IS NULL`, cid).Scan(&owners); err != nil {
			return err
		}
		if owners == 1 {
			return errs.ErrConflict
		}
	}
	_, err = tx.Exec(ctx, `UPDATE conversation_members SET role=$3,modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, target, newRole)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
