package repository

import (
	"context"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

type MessageInteractionRepository interface {
	GetVisibleMessage(context.Context, string, string) (*model.Message, error)
	SetReaction(context.Context, string, string, string, bool) ([]model.ReactionAggregate, bool, error)
}

type ConversationMessageRepository interface {
	CreateConversationMessage(context.Context, *model.Message) ([]string, error)
	GetMessagesByConversation(context.Context, string, string, int, int) (model.Messages, error)
	ActiveRecipients(context.Context, string, string) ([]string, error)
}

type MentionMessageRepository interface {
	CreateConversationMessageWithMentions(context.Context, *model.Message, []model.MessageMention) ([]string, error)
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
	_, err := r.CreateConversationMessage(ctx, msg)
	return err
}

func (r *MessageRepositoryImpl) CreateConversationMessage(ctx context.Context, msg *model.Message) ([]string, error) {
	return r.CreateConversationMessageWithMentions(ctx, msg, nil)
}

func (r *MessageRepositoryImpl) CreateConversationMessageWithMentions(ctx context.Context, msg *model.Message, mentions []model.MessageMention) ([]string, error) {
	q := `
		INSERT INTO messages (id, sender_id, receiver_id, body, is_group, created_at, modified_at, conversation_id, client_message_id, reply_to_message_id, forwarded_from_message_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9,'')::uuid, NULLIF($10,'')::uuid, NULLIF($11,'')::uuid)
	`
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Wrap("repository.MessageRepository.CreateMessage", err)
	}
	defer tx.Rollback(ctx)
	var kind string
	var receiver *string
	var directReceiver *string
	err = tx.QueryRow(ctx, `SELECT c.kind,CASE WHEN c.kind='direct' THEN CASE WHEN c.user_one_id=$2 THEN c.user_two_id::text ELSE c.user_one_id::text END END FROM conversations c JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$2 AND cm.left_at IS NULL WHERE c.id=$1 FOR UPDATE OF c,cm`, msg.ConversationID, msg.SenderID).Scan(&kind, &directReceiver)
	if err == pgx.ErrNoRows {
		return nil, errs.ErrNotMember
	}
	if err != nil {
		return nil, errs.Wrap("repository.MessageRepository.AuthorizeCreate", err)
	}
	if kind == "group" {
		msg.IsGroup = true
		msg.ReceiverID = ""
	} else {
		if directReceiver == nil || *directReceiver == msg.SenderID {
			return nil, errs.ErrSelfAction
		}
		msg.ReceiverID = *directReceiver
		receiver = directReceiver
	}
	utf16Length := 0
	for _, char := range msg.Body {
		utf16Length++
		if char > 0xffff {
			utf16Length++
		}
	}
	lastEnd := 0
	boundaries := map[int]bool{0: true}
	position := 0
	for _, char := range msg.Body {
		position++
		if char > 0xffff {
			position++
		}
		boundaries[position] = true
	}
	for i, mention := range mentions {
		end := mention.Offset + mention.Length
		if mention.Offset < 0 || mention.Length <= 0 || mention.Offset > utf16Length-mention.Length || !boundaries[mention.Offset] || !boundaries[end] || (i > 0 && mention.Offset < lastEnd) {
			return nil, errs.ErrValidation
		}
		lastEnd = end
	}
	_, err = tx.Exec(ctx, q, msg.ID, msg.SenderID, receiver, msg.Body, msg.IsGroup, msg.CreatedAt, msg.ModifiedAt, msg.ConversationID, msg.ClientMessageID, msg.ReplyToMessageID, msg.ForwardedFromMessageID)
	if err != nil {
		return nil, errs.Wrap("repository.MessageRepository.CreateMessage", err)
	}
	for _, mention := range mentions {
		var inserted string
		err = tx.QueryRow(ctx, `INSERT INTO message_mentions(message_id,user_id,kind,start_offset,length)
			SELECT $1,NULLIF($2,'')::uuid,$3,$4,$5
			WHERE ($3='user' AND EXISTS(SELECT 1 FROM conversation_members cm JOIN users u ON u.id=cm.user_id AND u.deleted_at IS NULL WHERE cm.conversation_id=$6 AND cm.user_id=NULLIF($2,'')::uuid AND cm.left_at IS NULL))
			   OR ($3='everyone' AND $7='group' AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=$6 AND cm.user_id=$8 AND cm.left_at IS NULL AND cm.role IN ('owner','admin')))
			RETURNING message_id`, msg.ID, mention.UserID, mention.Kind, mention.Offset, mention.Length, msg.ConversationID, kind, msg.SenderID).Scan(&inserted)
		if err == pgx.ErrNoRows {
			return nil, errs.ErrForbidden
		}
		if err != nil {
			return nil, errs.Wrap("repository.Message.Mention", err)
		}
	}
	rows, err := tx.Query(ctx, `INSERT INTO message_deliveries(message_id,recipient_id,status,status_at) SELECT $1,cm.user_id,'sent',$3 FROM conversation_members cm WHERE cm.conversation_id=$2 AND cm.left_at IS NULL AND cm.user_id<>$4 ON CONFLICT DO NOTHING RETURNING recipient_id`, msg.ID, msg.ConversationID, msg.CreatedAt, msg.SenderID)
	if err != nil {
		return nil, errs.Wrap("repository.MessageRepository.CreateDelivery", err)
	}
	var recipients []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, errs.Wrap("repository.MessageRepository.CreateDelivery", err)
		}
		recipients = append(recipients, id)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, errs.Wrap("repository.MessageRepository.CreateDelivery", err)
	}
	msg.Mentions = mentions
	notificationRows, err := tx.Query(ctx, `
		WITH eligible AS (
			SELECT cm.user_id,
				CASE WHEN EXISTS(SELECT 1 FROM message_mentions mm WHERE mm.message_id=$1 AND (mm.kind='everyone' OR mm.user_id=cm.user_id)) THEN 'mention'
				     WHEN NULLIF($5,'')::uuid IS NOT NULL AND EXISTS(SELECT 1 FROM messages parent WHERE parent.id=NULLIF($5,'')::uuid AND parent.sender_id=cm.user_id) THEN 'reply'
				     ELSE 'message' END AS notification_type
			FROM conversation_members cm
			LEFT JOIN conversation_user_state state ON state.conversation_id=cm.conversation_id AND state.user_id=cm.user_id
			LEFT JOIN notification_preferences pref ON pref.user_id=cm.user_id
			WHERE cm.conversation_id=$2 AND cm.left_at IS NULL AND cm.user_id<>$3
			  AND COALESCE(state.muted_until<=NOW(),TRUE)
		), allowed AS (
			SELECT e.* FROM eligible e LEFT JOIN notification_preferences p ON p.user_id=e.user_id
			WHERE (e.notification_type='message' AND COALESCE(p.message_enabled,TRUE))
			   OR (e.notification_type='reply' AND COALESCE(p.reply_enabled,TRUE))
			   OR (e.notification_type='mention' AND COALESCE(p.mention_enabled,TRUE))
		)
		INSERT INTO notifications(id,recipient_id,actor_id,type,conversation_id,message_id,payload,dedupe_key,created_at)
		SELECT gen_random_uuid(),user_id,$3,notification_type,$2,$1,jsonb_build_object('content',$4::text),$1::text||':'||user_id::text||':'||notification_type,$6::timestamptz
		FROM allowed ON CONFLICT(dedupe_key) DO NOTHING
		RETURNING id,recipient_id,actor_id,type,conversation_id,message_id,payload,created_at,read_at`, msg.ID, msg.ConversationID, msg.SenderID, msg.Body, msg.ReplyToMessageID, msg.CreatedAt)
	if err != nil {
		return nil, errs.Wrap("repository.Message.Notifications", err)
	}
	for notificationRows.Next() {
		n, scanErr := scanNotification(notificationRows)
		if scanErr != nil {
			notificationRows.Close()
			return nil, scanErr
		}
		msg.CreatedNotifications = append(msg.CreatedNotifications, *n)
	}
	notificationRows.Close()
	if err = notificationRows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errs.Wrap("repository.MessageRepository.CreateMessage", err)
	}
	msg.Status = "sent"
	return recipients, nil
}
func (r *MessageRepositoryImpl) GetMessage(ctx context.Context, id string) (*model.Message, error) {
	var m model.Message
	e := r.db.QueryRow(ctx, `SELECT m.id,m.sender_id,COALESCE(m.receiver_id::text,''),m.body,m.is_group,m.created_at,m.modified_at,m.conversation_id,COALESCE(m.client_message_id::text,''),m.edited_at,COALESCE(m.reply_to_message_id::text,''),COALESCE(m.forwarded_from_message_id::text,''),COALESCE(d.status,'sent') FROM messages m LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=m.receiver_id WHERE m.id=$1 AND m.deleted_at IS NULL`, id).Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Body, &m.IsGroup, &m.CreatedAt, &m.ModifiedAt, &m.ConversationID, &m.ClientMessageID, &m.EditedAt, &m.ReplyToMessageID, &m.ForwardedFromMessageID, &m.Status)
	if e == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return &m, errs.Wrap("repository.Message.Get", e)
}

func (r *MessageRepositoryImpl) GetMessagesByConversation(ctx context.Context, cid, uid string, limit, offset int) (model.Messages, error) {
	var joined time.Time
	if err := r.db.QueryRow(ctx, `SELECT joined_at FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL`, cid, uid).Scan(&joined); err == pgx.ErrNoRows {
		return nil, errs.ErrNotMember
	} else if err != nil {
		return nil, errs.Wrap("repository.Message.History", err)
	}
	rows, err := r.db.Query(ctx, `SELECT m.id,m.sender_id,COALESCE(m.receiver_id::text,''),m.body,m.is_group,m.created_at,m.modified_at,m.conversation_id,COALESCE(m.client_message_id::text,''),m.edited_at,COALESCE(m.reply_to_message_id::text,''),COALESCE(m.forwarded_from_message_id::text,''),COALESCE(d.status,'sent') FROM conversation_members cm JOIN conversations c ON c.id=cm.conversation_id JOIN messages m ON m.conversation_id=c.id LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=$2 WHERE cm.conversation_id=$1 AND cm.user_id=$2 AND cm.left_at IS NULL AND m.deleted_at IS NULL AND (c.kind='direct' OR m.created_at>=cm.joined_at) ORDER BY m.created_at,m.id LIMIT $3 OFFSET $4`, cid, uid, limit, offset)
	if err != nil {
		return nil, errs.Wrap("repository.Message.History", err)
	}
	defer rows.Close()
	out := model.Messages{}
	for rows.Next() {
		var m model.Message
		if err = rows.Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Body, &m.IsGroup, &m.CreatedAt, &m.ModifiedAt, &m.ConversationID, &m.ClientMessageID, &m.EditedAt, &m.ReplyToMessageID, &m.ForwardedFromMessageID, &m.Status); err != nil {
			return nil, errs.Wrap("repository.Message.History", err)
		}
		out = append(out, &m)
	}
	if err = rows.Err(); err != nil {
		return nil, errs.Wrap("repository.Message.History", err)
	}
	if err = r.hydrateInteractions(ctx, uid, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MessageRepositoryImpl) GetVisibleMessage(ctx context.Context, id, uid string) (*model.Message, error) {
	var m model.Message
	err := r.db.QueryRow(ctx, `SELECT m.id,m.sender_id,COALESCE(m.receiver_id::text,''),m.body,m.is_group,m.created_at,m.modified_at,m.conversation_id,COALESCE(m.client_message_id::text,''),m.edited_at,COALESCE(m.reply_to_message_id::text,''),COALESCE(m.forwarded_from_message_id::text,''),'sent' FROM messages m JOIN conversations c ON c.id=m.conversation_id JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$2 AND cm.left_at IS NULL WHERE m.id=$1 AND m.deleted_at IS NULL AND (c.kind='direct' OR m.created_at>=cm.joined_at)`, id, uid).Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Body, &m.IsGroup, &m.CreatedAt, &m.ModifiedAt, &m.ConversationID, &m.ClientMessageID, &m.EditedAt, &m.ReplyToMessageID, &m.ForwardedFromMessageID, &m.Status)
	if err == pgx.ErrNoRows {
		return nil, errs.ErrNotFound
	}
	return &m, errs.Wrap("repository.Message.Visible", err)
}

func (r *MessageRepositoryImpl) SetReaction(ctx context.Context, messageID, uid, reaction string, add bool) ([]model.ReactionAggregate, bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)
	var visible bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages m JOIN conversations c ON c.id=m.conversation_id JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$2 AND cm.left_at IS NULL WHERE m.id=$1 AND m.deleted_at IS NULL AND (c.kind='direct' OR m.created_at>=cm.joined_at))`, messageID, uid).Scan(&visible)
	if err != nil {
		return nil, false, errs.Wrap("repository.Message.ReactionVisible", err)
	}
	if !visible {
		return nil, false, errs.ErrNotFound
	}
	var tag pgconn.CommandTag
	if add {
		tag, err = tx.Exec(ctx, `INSERT INTO message_reactions(message_id,user_id,reaction) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, messageID, uid, reaction)
	} else {
		tag, err = tx.Exec(ctx, `DELETE FROM message_reactions WHERE message_id=$1 AND user_id=$2 AND reaction=$3`, messageID, uid, reaction)
	}
	if err != nil {
		return nil, false, errs.Wrap("repository.Message.Reaction", err)
	}
	aggregates, err := reactionAggregates(ctx, tx, messageID, uid)
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return aggregates, tag.RowsAffected() > 0, nil
}

type reactionQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func reactionAggregates(ctx context.Context, q reactionQuerier, messageID, uid string) ([]model.ReactionAggregate, error) {
	rows, err := q.Query(ctx, `SELECT reaction,count(*),bool_or(user_id=$2) FROM message_reactions WHERE message_id=$1 GROUP BY reaction ORDER BY min(created_at),reaction`, messageID, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ReactionAggregate{}
	for rows.Next() {
		var a model.ReactionAggregate
		if err = rows.Scan(&a.Reaction, &a.Count, &a.ReactedByMe); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r *MessageRepositoryImpl) hydrateInteractions(ctx context.Context, uid string, messages model.Messages) error {
	for _, m := range messages {
		a, err := reactionAggregates(ctx, r.db, m.ID, uid)
		if err != nil {
			return err
		}
		m.Reactions = a
		preview := func(id string) *model.MessagePreview {
			if id == "" {
				return nil
			}
			source, err := r.GetVisibleMessage(ctx, id, uid)
			if err != nil {
				return nil
			}
			return &model.MessagePreview{ID: source.ID, SenderID: source.SenderID, Content: source.Body}
		}
		m.ReplyTo = preview(m.ReplyToMessageID)
		m.ForwardedFrom = preview(m.ForwardedFromMessageID)
		rows, mentionErr := r.db.Query(ctx, `SELECT kind,COALESCE(user_id::text,''),start_offset,length FROM message_mentions WHERE message_id=$1 ORDER BY start_offset`, m.ID)
		if mentionErr != nil {
			return mentionErr
		}
		m.Mentions = []model.MessageMention{}
		for rows.Next() {
			var mention model.MessageMention
			if mentionErr = rows.Scan(&mention.Kind, &mention.UserID, &mention.Offset, &mention.Length); mentionErr != nil {
				rows.Close()
				return mentionErr
			}
			m.Mentions = append(m.Mentions, mention)
		}
		rows.Close()
	}
	return nil
}

func (r *MessageRepositoryImpl) ActiveRecipients(ctx context.Context, cid, actor string) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT peers.user_id FROM conversation_members actor JOIN conversation_members peers ON peers.conversation_id=actor.conversation_id AND peers.left_at IS NULL AND peers.user_id<>actor.user_id WHERE actor.conversation_id=$1 AND actor.user_id=$2 AND actor.left_at IS NULL ORDER BY peers.user_id`, cid, actor)
	if err != nil {
		return nil, errs.Wrap("repository.Message.Recipients", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		var member bool
		if err = r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 AND left_at IS NULL)`, cid, actor).Scan(&member); err != nil {
			return nil, err
		}
		if !member {
			return nil, errs.ErrNotMember
		}
	}
	return out, nil
}
func (r *MessageRepositoryImpl) GetByClientMessageID(ctx context.Context, cid, sender, conversation string) (*model.Message, error) {
	var m model.Message
	e := r.db.QueryRow(ctx, `SELECT m.id,m.sender_id,COALESCE(m.receiver_id::text,''),m.body,m.is_group,m.created_at,m.modified_at,m.conversation_id,COALESCE(m.client_message_id::text,''),m.edited_at,COALESCE(m.reply_to_message_id::text,''),COALESCE(m.forwarded_from_message_id::text,''),COALESCE(d.status,'sent') FROM messages m LEFT JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=m.receiver_id WHERE m.client_message_id=$1 AND m.sender_id=$2 AND m.conversation_id=$3`, cid, sender, conversation).Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Body, &m.IsGroup, &m.CreatedAt, &m.ModifiedAt, &m.ConversationID, &m.ClientMessageID, &m.EditedAt, &m.ReplyToMessageID, &m.ForwardedFromMessageID, &m.Status)
	if e == pgx.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return &m, errs.Wrap("repository.Message.GetByClientMessageID", e)
	}
	if e = r.hydrateInteractions(ctx, sender, model.Messages{&m}); e != nil {
		return nil, e
	}
	return &m, nil
}

func (r *MessageRepositoryImpl) GetMessagesByReceiver(ctx context.Context, receiverID string, limit, offset int) (model.Messages, error) {
	query := `
		SELECT m.id, m.sender_id, m.receiver_id, m.body, m.is_group, m.created_at, m.modified_at, m.conversation_id, COALESCE(m.client_message_id::text,''), m.edited_at, COALESCE(m.reply_to_message_id::text,''), COALESCE(m.forwarded_from_message_id::text,''), COALESCE(d.status,'sent')
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
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Body, &msg.IsGroup, &msg.CreatedAt, &msg.ModifiedAt, &msg.ConversationID, &msg.ClientMessageID, &msg.EditedAt, &msg.ReplyToMessageID, &msg.ForwardedFromMessageID, &msg.Status); err != nil {
			return nil, errs.Wrap("repository.MessageRepository.GetMessagesByReceiver", err)
		}
		messages = append(messages, &msg)
	}
	return messages, errs.Wrap("repository.MessageRepository.GetMessagesByReceiver", rows.Err())
}

func (r *MessageRepositoryImpl) GetMessagesBetweenUsers(ctx context.Context, senderID, receiverID string, limit, offset int) (model.Messages, error) {
	query := `
		SELECT m.id, m.sender_id, m.receiver_id, m.body, m.is_group, m.created_at, m.modified_at, m.conversation_id, COALESCE(m.client_message_id::text,''), m.edited_at, COALESCE(m.reply_to_message_id::text,''), COALESCE(m.forwarded_from_message_id::text,''), COALESCE(d.status,'sent')
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
		if err := rows.Scan(&msg.ID, &msg.SenderID, &msg.ReceiverID, &msg.Body, &msg.IsGroup, &msg.CreatedAt, &msg.ModifiedAt, &msg.ConversationID, &msg.ClientMessageID, &msg.EditedAt, &msg.ReplyToMessageID, &msg.ForwardedFromMessageID, &msg.Status); err != nil {
			return nil, errs.Wrap("repository.MessageRepository.GetMessagesBetweenUsers", err)
		}
		messages = append(messages, &msg)
	}
	if err = rows.Err(); err != nil {
		return nil, errs.Wrap("repository.MessageRepository.GetMessagesBetweenUsers", err)
	}
	if err = r.hydrateInteractions(ctx, senderID, messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *MessageRepositoryImpl) EditMessage(ctx context.Context, id, senderID, body string) error {
	res, e := r.db.Exec(ctx, `UPDATE messages m SET body=$3,edited_at=NOW(),modified_at=NOW() WHERE m.id=$1 AND m.sender_id=$2 AND m.deleted_at IS NULL AND (NOT m.is_group OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND cm.left_at IS NULL))`, id, senderID, body)
	if e == nil && res.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return errs.Wrap("repository.Message.Edit", e)
}
func (r *MessageRepositoryImpl) DeleteMessage(ctx context.Context, id, senderID string) error {
	res, e := r.db.Exec(ctx, `UPDATE messages m SET deleted_at=NOW(),modified_at=NOW() WHERE m.id=$1 AND m.sender_id=$2 AND m.deleted_at IS NULL AND (NOT m.is_group OR EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND cm.left_at IS NULL))`, id, senderID)
	if e == nil && res.RowsAffected() == 0 {
		return errs.ErrNotFound
	}
	return errs.Wrap("repository.Message.Delete", e)
}
func (r *MessageRepositoryImpl) CreateReply(ctx context.Context, m *model.Message) error {
	return r.CreateMessage(ctx, m)
}
func (r *MessageRepositoryImpl) MarkDelivery(ctx context.Context, messageID, recipient, status string, at time.Time) error {
	res, e := r.db.Exec(ctx, `UPDATE message_deliveries d SET status=$3,status_at=$4,failure_code=NULL
		FROM messages m JOIN conversations c ON c.id=m.conversation_id JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND cm.left_at IS NULL
		WHERE d.message_id=m.id AND d.recipient_id=$2 AND m.id=$1 AND m.deleted_at IS NULL AND m.sender_id<>$2
		  AND (c.kind='direct' OR m.created_at>=cm.joined_at)
		  AND CASE d.status WHEN 'read' THEN 3 WHEN 'delivered' THEN 2 WHEN 'sent' THEN 1 ELSE 0 END < CASE $3 WHEN 'read' THEN 3 WHEN 'delivered' THEN 2 WHEN 'sent' THEN 1 ELSE 0 END`, messageID, recipient, status, at)
	if e == nil && res.RowsAffected() == 0 {
		var eligible bool
		if qe := r.db.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM message_deliveries d
			JOIN messages m ON m.id=d.message_id
			JOIN conversations c ON c.id=m.conversation_id
			JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND cm.left_at IS NULL
			WHERE d.message_id=$1 AND d.recipient_id=$2 AND m.deleted_at IS NULL AND m.sender_id<>$2
			  AND (c.kind='direct' OR m.created_at>=cm.joined_at))`, messageID, recipient).Scan(&eligible); qe != nil {
			return errs.Wrap("repository.Message.Delivery", qe)
		}
		if !eligible {
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
	if e = tx.QueryRow(ctx, `SELECT m.created_at FROM messages m JOIN conversations c ON c.id=m.conversation_id JOIN message_deliveries d ON d.message_id=m.id AND d.recipient_id=$3 JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$3 AND cm.left_at IS NULL WHERE m.id=$1 AND m.conversation_id=$2 AND m.deleted_at IS NULL AND (c.kind='direct' OR m.created_at>=cm.joined_at)`, messageID, cid, uid).Scan(&created); e == pgx.ErrNoRows {
		return errs.ErrNotFound
	} else if e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	if _, e = tx.Exec(ctx, `UPDATE message_deliveries d SET status='read',status_at=$4,failure_code=NULL FROM messages m, conversations c, conversation_members cm WHERE d.message_id=m.id AND c.id=m.conversation_id AND cm.conversation_id=m.conversation_id AND cm.user_id=$2 AND cm.left_at IS NULL AND d.recipient_id=$2 AND m.conversation_id=$1 AND (c.kind='direct' OR m.created_at>=cm.joined_at) AND (m.created_at,m.id) <= ($3,$5::uuid) AND d.status <> 'read'`, cid, uid, created, at, messageID); e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	if _, e = tx.Exec(ctx, `INSERT INTO conversation_read_state(conversation_id,user_id,last_read_message_id,last_read_at) VALUES($1,$2,$3,$4) ON CONFLICT(conversation_id,user_id) DO UPDATE SET last_read_message_id=EXCLUDED.last_read_message_id,last_read_at=EXCLUDED.last_read_at WHERE conversation_read_state.last_read_message_id IS NULL OR (SELECT (created_at,id) FROM messages WHERE id=EXCLUDED.last_read_message_id) >= (SELECT (created_at,id) FROM messages WHERE id=conversation_read_state.last_read_message_id)`, cid, uid, messageID, at); e != nil {
		return errs.Wrap("repository.Message.Read", e)
	}
	return errs.Wrap("repository.Message.Read", tx.Commit(ctx))
}
func (r *MessageRepositoryImpl) UnreadSummary(ctx context.Context, uid string) (map[string]int, error) {
	rows, e := r.db.Query(ctx, `SELECT m.conversation_id,COUNT(*) FROM message_deliveries d JOIN messages m ON m.id=d.message_id JOIN conversations c ON c.id=m.conversation_id JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$1 AND cm.left_at IS NULL JOIN users sender ON sender.id=m.sender_id AND sender.deleted_at IS NULL WHERE d.recipient_id=$1 AND m.sender_id<>$1 AND d.status <> 'read' AND m.deleted_at IS NULL AND (c.kind='direct' OR m.created_at>=cm.joined_at) GROUP BY m.conversation_id`, uid)
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
			       max(m.created_at) AS created_at, c.kind, cm.joined_at
			FROM messages m
			JOIN conversations c ON c.id=m.conversation_id
			JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$1 AND cm.left_at IS NULL
			LEFT JOIN conversation_user_state s ON s.conversation_id=m.conversation_id AND s.user_id=$1
			WHERE EXISTS(SELECT 1 FROM message_deliveries md WHERE md.message_id=m.id AND md.recipient_id=$1) AND m.deleted_at IS NULL
			  AND (c.kind='direct' OR m.created_at>=cm.joined_at)
			  AND s.archived_at IS NULL AND s.hidden_at IS NULL
			GROUP BY m.conversation_id,c.kind,cm.joined_at
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
	if _, err = tx.Exec(ctx, latest+`UPDATE message_deliveries d SET status='read',status_at=$2,failure_code=NULL FROM messages m,latest l WHERE d.message_id=m.id AND d.recipient_id=$1 AND m.conversation_id=l.conversation_id AND (l.kind='direct' OR m.created_at>=l.joined_at) AND (m.created_at,m.id)<=(l.created_at,l.message_id) AND d.status <> 'read'`, uid, at); err != nil {
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
