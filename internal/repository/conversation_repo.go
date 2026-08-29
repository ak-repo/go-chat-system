package repository

import (
	"context"
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
type ConversationRepositoryImpl struct{ db *pgxpool.Pool }

func NewConversationRepository(db *pgxpool.Pool) *ConversationRepositoryImpl {
	return &ConversationRepositoryImpl{db: db}
}
func scanConversation(row pgx.Row) (*model.Conversation, error) {
	var c model.Conversation
	err := row.Scan(&c.ID, &c.Kind, &c.UserOneID, &c.UserTwoID, &c.CreatedAt, &c.ModifiedAt)
	return &c, err
}
func (r *ConversationRepositoryImpl) CreateOrGetDirect(ctx context.Context, a, b string) (*model.Conversation, error) {
	id := uuid.NewString()
	_, err := r.db.Exec(ctx, `INSERT INTO conversations(id,kind,user_one_id,user_two_id) VALUES($1,'direct',LEAST($2::uuid,$3::uuid),GREATEST($2::uuid,$3::uuid)) ON CONFLICT DO NOTHING`, id, a, b)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.Create", err)
	}
	_, err = r.db.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id) SELECT c.id,x.user_id FROM conversations c CROSS JOIN (VALUES($2::uuid),($3::uuid)) x(user_id) WHERE c.user_one_id=LEAST($2::uuid,$3::uuid) AND c.user_two_id=GREATEST($2::uuid,$3::uuid) ON CONFLICT DO NOTHING`, id, a, b)
	if err != nil {
		return nil, errs.Wrap("repository.Conversation.Member", err)
	}
	return r.Get(ctx, id, a)
}
func (r *ConversationRepositoryImpl) Get(ctx context.Context, id, uid string) (*model.Conversation, error) {
	c, e := scanConversation(r.db.QueryRow(ctx, `SELECT c.id,c.kind,c.user_one_id,c.user_two_id,c.created_at,c.modified_at FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id AND m.user_id=$2 AND m.left_at IS NULL WHERE c.id=$1`, id, uid))
	if e == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return c, errs.Wrap("repository.Conversation.Get", e)
}
func (r *ConversationRepositoryImpl) List(ctx context.Context, uid string, limit, offset int) ([]*model.Conversation, error) {
	rows, e := r.db.Query(ctx, `SELECT c.id,c.kind,c.user_one_id,c.user_two_id,c.created_at,c.modified_at FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE m.user_id=$1 AND m.left_at IS NULL ORDER BY c.modified_at DESC,c.id LIMIT $2 OFFSET $3`, uid, limit, offset)
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
func (r *ConversationRepositoryImpl) IsMember(ctx context.Context, cid, uid string) (bool, error) {
	var ok bool
	e := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, cid, uid).Scan(&ok)
	return ok, errs.Wrap("repository.Conversation.Member", e)
}
