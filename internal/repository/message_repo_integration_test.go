//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
)

func TestMessageRepositoryPersistsMutationsAndReadState(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	conversation, err := NewConversationRepository(db).CreateOrGetDirect(context.Background(), ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	m := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[0], Body: "hello", ConversationID: conversation.ID, ClientMessageID: uuid.NewString(), CreatedAt: now, ModifiedAt: now}
	if err := r.CreateMessage(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetMessage(context.Background(), m.ID)
	if err != nil || got.Body != "hello" || got.ReceiverID != ids[1] {
		t.Fatalf("get message: %#v, %v", got, err)
	}
	if err := r.EditMessage(context.Background(), m.ID, ids[0], "edited"); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkDelivery(context.Background(), m.ID, ids[1], "delivered", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkRead(context.Background(), conversation.ID, ids[1], m.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetMessage(context.Background(), m.ID)
	if err != nil || got.Status != "read" {
		t.Fatalf("persisted status not returned: %#v, %v", got, err)
	}
	if unread, err := r.UnreadSummary(context.Background(), ids[1]); err != nil || len(unread) != 0 {
		t.Fatalf("unread summary after read: %#v, %v", unread, err)
	}
	if err := r.DeleteMessage(context.Background(), m.ID, ids[0]); err != nil {
		t.Fatal(err)
	}
	if deleted, err := r.GetMessage(context.Background(), m.ID); err == nil || deleted != nil {
		t.Fatalf("deleted message should be hidden: %#v, %v", deleted, err)
	}
}

func TestGroupMessageCreateHistoryAndDeliveries(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 4)
	cid := uuid.NewString()
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT INTO conversations(id,kind,name,creator_id) VALUES($1,'group','group',$2)`, cid, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,CASE WHEN x=$2 THEN 'owner' ELSE 'member' END,$2 FROM unnest($3::uuid[]) x`, cid, ids[0], ids[:3]); err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	m := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[3], Body: "group hello", ConversationID: cid, CreatedAt: now, ModifiedAt: now}
	recipients, err := r.CreateConversationMessage(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsGroup || m.ReceiverID != "" || len(recipients) != 2 {
		t.Fatalf("unexpected group create: %#v recipients=%v", m, recipients)
	}
	var deliveries int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM message_deliveries WHERE message_id=$1`, m.ID).Scan(&deliveries); err != nil || deliveries != 2 {
		t.Fatalf("deliveries=%d err=%v", deliveries, err)
	}
	history, err := r.GetMessagesByConversation(ctx, cid, ids[1], 50, 0)
	if err != nil || len(history) != 1 {
		t.Fatalf("history=%v err=%v", history, err)
	}
	if err = r.MarkDelivery(ctx, m.ID, ids[2], "delivered", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = r.MarkRead(ctx, cid, ids[1], m.ID, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	var firstStatus, secondStatus string
	if err = db.QueryRow(ctx, `SELECT status FROM message_deliveries WHERE message_id=$1 AND recipient_id=$2`, m.ID, ids[1]).Scan(&firstStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `SELECT status FROM message_deliveries WHERE message_id=$1 AND recipient_id=$2`, m.ID, ids[2]).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "read" || secondStatus != "delivered" {
		t.Fatalf("delivery state was not independent: %s %s", firstStatus, secondStatus)
	}
	if _, err = r.GetMessagesByConversation(ctx, cid, ids[3], 50, 0); !errors.Is(err, errs.ErrNotMember) {
		t.Fatalf("expected non-member history rejection, got %v", err)
	}
	outsider := &model.Message{ID: uuid.NewString(), SenderID: ids[3], Body: "forbidden", ConversationID: cid, CreatedAt: now, ModifiedAt: now}
	if _, err = r.CreateConversationMessage(ctx, outsider); !errors.Is(err, errs.ErrNotMember) {
		t.Fatalf("expected non-member rejection, got %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE conversation_members SET left_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetMessagesByConversation(ctx, cid, ids[1], 50, 0); !errors.Is(err, errs.ErrNotMember) {
		t.Fatalf("expected departed-member rejection, got %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE conversation_members SET joined_at=NOW()+INTERVAL '1 minute',modified_at=NOW()+INTERVAL '1 minute',left_at=NULL WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1]); err != nil {
		t.Fatal(err)
	}
	history, err = r.GetMessagesByConversation(ctx, cid, ids[1], 50, 0)
	if err != nil || len(history) != 0 {
		t.Fatalf("rejoined history=%v err=%v", history, err)
	}
}

func TestGroupReceiptsRequireCurrentEligibleDelivery(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 3)
	ctx, cid := context.Background(), uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO conversations(id,kind,name,creator_id) VALUES($1,'group','receipts',$2)`, cid, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,CASE WHEN x=$2 THEN 'owner' ELSE 'member' END,$2 FROM unnest($3::uuid[]) x`, cid, ids[0], ids[:2]); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	message := &model.Message{ID: uuid.NewString(), SenderID: ids[0], Body: "before rejoin", ConversationID: cid, CreatedAt: now, ModifiedAt: now}
	if _, err := repo.CreateConversationMessage(ctx, message); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE conversation_members SET left_at=NOW(),modified_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkDelivery(ctx, message.ID, ids[1], "delivered", now.Add(time.Second)); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("departed member updated delivery: %v", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM message_deliveries WHERE message_id=$1 AND recipient_id=$2`, message.ID, ids[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE conversation_members SET joined_at=NOW()+INTERVAL '1 minute',modified_at=NOW()+INTERVAL '1 minute',left_at=NULL WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkDelivery(ctx, message.ID, ids[1], "delivered", now.Add(2*time.Second)); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("rejoined member synthesized old delivery: %v", err)
	}
	if err := repo.MarkRead(ctx, cid, ids[1], message.ID, now.Add(3*time.Second)); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("rejoined member read old message: %v", err)
	}
	unread, err := repo.UnreadSummary(ctx, ids[1])
	if err != nil || unread[cid] != 0 {
		t.Fatalf("old message appeared unread: %#v %v", unread, err)
	}
	var deliveries int
	if err = db.QueryRow(ctx, `SELECT count(*) FROM message_deliveries WHERE message_id=$1 AND recipient_id=$2`, message.ID, ids[1]).Scan(&deliveries); err != nil || deliveries != 0 {
		t.Fatalf("ineligible delivery was synthesized: %d %v", deliveries, err)
	}
}

func TestMessageReferencesAndNotificationsStayInConversation(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 3)
	ctx := context.Background()
	first, err := NewConversationRepository(db).CreateOrGetDirect(ctx, ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewConversationRepository(db).CreateOrGetDirect(ctx, ids[0], ids[2])
	if err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	parent := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[1], Body: "parent", ConversationID: first.ID, CreatedAt: now, ModifiedAt: now}
	if err = repo.CreateMessage(ctx, parent); err != nil {
		t.Fatal(err)
	}
	wrongReply := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[2], Body: "wrong", ConversationID: second.ID, ReplyToMessageID: parent.ID, CreatedAt: now, ModifiedAt: now}
	if err = repo.CreateMessage(ctx, wrongReply); err == nil {
		t.Fatal("cross-conversation reply was persisted")
	}
	if _, err = db.Exec(ctx, `INSERT INTO notifications(id,recipient_id,actor_id,type,conversation_id,message_id,dedupe_key) VALUES($1,$2,$3,'message',$4,$5,$6)`, uuid.NewString(), ids[2], ids[0], second.ID, parent.ID, uuid.NewString()); err == nil {
		t.Fatal("notification paired a message with the wrong conversation")
	}
}

func TestMentionsAndNotificationsAreTransactionalAndHonorPolicy(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 3)
	ctx, cid := context.Background(), uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO conversations(id,kind,name,creator_id) VALUES($1,'group','notices',$2)`, cid, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,CASE WHEN x=$2 THEN 'owner' ELSE 'member' END,$2 FROM unnest($3::uuid[]) x`, cid, ids[0], ids); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	message := &model.Message{ID: uuid.NewString(), SenderID: ids[0], Body: "hello @everyone", ConversationID: cid, CreatedAt: now, ModifiedAt: now}
	if _, err := repo.CreateConversationMessageWithMentions(ctx, message, []model.MessageMention{{Kind: "everyone", Offset: 6, Length: 9}}); err != nil {
		t.Fatal(err)
	}
	if len(message.CreatedNotifications) != 2 || message.CreatedNotifications[0].Type != "mention" {
		t.Fatalf("unexpected notifications: %#v", message.CreatedNotifications)
	}
	history, err := repo.GetMessagesByConversation(ctx, cid, ids[1], 10, 0)
	if err != nil || len(history) != 1 || len(history[0].Mentions) != 1 {
		t.Fatalf("mention history=%#v err=%v", history, err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO conversation_user_state(conversation_id,user_id,muted_until) VALUES($1,$2,NOW()+INTERVAL '1 hour')`, cid, ids[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO notification_preferences(user_id,mention_enabled) VALUES($1,FALSE)`, ids[2]); err != nil {
		t.Fatal(err)
	}
	muted := &model.Message{ID: uuid.NewString(), SenderID: ids[0], Body: "again @everyone", ConversationID: cid, CreatedAt: now.Add(time.Second), ModifiedAt: now.Add(time.Second)}
	if _, err = repo.CreateConversationMessageWithMentions(ctx, muted, []model.MessageMention{{Kind: "everyone", Offset: 6, Length: 9}}); err != nil || len(muted.CreatedNotifications) != 0 {
		t.Fatalf("suppressed notifications=%#v err=%v", muted.CreatedNotifications, err)
	}
	forbidden := &model.Message{ID: uuid.NewString(), SenderID: ids[1], Body: "bad @everyone", ConversationID: cid, CreatedAt: now.Add(2 * time.Second), ModifiedAt: now.Add(2 * time.Second)}
	if _, err = repo.CreateConversationMessageWithMentions(ctx, forbidden, []model.MessageMention{{Kind: "everyone", Offset: 4, Length: 9}}); !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected forbidden everyone mention, got %v", err)
	}
	var persisted bool
	if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1)`, forbidden.ID).Scan(&persisted); err != nil || persisted {
		t.Fatalf("forbidden message was not rolled back: persisted=%v err=%v", persisted, err)
	}
}

func TestMessageRepositoryReadCursorNeverMovesBackwards(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	c, err := NewConversationRepository(db).CreateOrGetDirect(context.Background(), ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	older := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[1], Body: "older", ConversationID: c.ID, CreatedAt: now, ModifiedAt: now}
	newer := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[1], Body: "newer", ConversationID: c.ID, CreatedAt: now.Add(time.Minute), ModifiedAt: now.Add(time.Minute)}
	if err := r.CreateMessage(context.Background(), older); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateMessage(context.Background(), newer); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkRead(context.Background(), c.ID, ids[1], newer.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkRead(context.Background(), c.ID, ids[1], older.ID, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var cursor string
	if err := db.QueryRow(context.Background(), `SELECT last_read_message_id FROM conversation_read_state WHERE conversation_id=$1 AND user_id=$2`, c.ID, ids[1]).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if cursor != newer.ID {
		t.Fatalf("read cursor moved backwards: got %s want %s", cursor, newer.ID)
	}
}

func TestMessageRepositoryMarkAllReadSkipsArchivedConversations(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 3)
	conversations := NewConversationRepository(db)
	active, err := conversations.CreateOrGetDirect(context.Background(), ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	archived, err := conversations.CreateOrGetDirect(context.Background(), ids[1], ids[2])
	if err != nil {
		t.Fatal(err)
	}
	archive := true
	if err := conversations.UpdatePreferences(context.Background(), archived.ID, ids[1], &archive, nil, nil); err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	for _, entry := range []struct {
		conversation     *model.Conversation
		sender, receiver string
	}{{active, ids[0], ids[1]}, {archived, ids[2], ids[1]}} {
		m := &model.Message{ID: uuid.NewString(), SenderID: entry.sender, ReceiverID: entry.receiver, Body: "unread", ConversationID: entry.conversation.ID, CreatedAt: now, ModifiedAt: now}
		if err := r.CreateMessage(context.Background(), m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.MarkAllActiveRead(context.Background(), ids[1], now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	unread, err := r.UnreadSummary(context.Background(), ids[1])
	if err != nil || unread[active.ID] != 0 || unread[archived.ID] != 1 {
		t.Fatalf("unexpected unread after mark-all: %#v, %v", unread, err)
	}
}

func TestMessageRepositoryMarkAllReadRespectsCurrentGroupJoin(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	ctx, cid := context.Background(), uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO conversations(id,kind,name,creator_id) VALUES($1,'group','rejoin',$2)`, cid, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,CASE WHEN x=$2 THEN 'owner' ELSE 'member' END,$2 FROM unnest($3::uuid[]) x`, cid, ids[0], ids); err != nil {
		t.Fatal(err)
	}
	repo := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	beforeRejoin := &model.Message{ID: uuid.NewString(), SenderID: ids[0], Body: "before rejoin", ConversationID: cid, CreatedAt: now, ModifiedAt: now}
	if _, err := repo.CreateConversationMessage(ctx, beforeRejoin); err != nil {
		t.Fatal(err)
	}
	joinedAt := now.Add(time.Minute)
	if _, err := db.Exec(ctx, `UPDATE conversation_members SET joined_at=$3,modified_at=$3,left_at=NULL WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1], joinedAt); err != nil {
		t.Fatal(err)
	}
	afterRejoin := &model.Message{ID: uuid.NewString(), SenderID: ids[0], Body: "after rejoin", ConversationID: cid, CreatedAt: now.Add(2 * time.Minute), ModifiedAt: now.Add(2 * time.Minute)}
	if _, err := repo.CreateConversationMessage(ctx, afterRejoin); err != nil {
		t.Fatal(err)
	}
	receipts, err := repo.MarkAllActiveRead(ctx, ids[1], now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || receipts[0].MessageID != afterRejoin.ID || receipts[0].ConversationID != cid {
		t.Fatalf("unexpected receipts after rejoin: %#v", receipts)
	}
	var oldStatus, newStatus, cursor string
	if err = db.QueryRow(ctx, `SELECT status FROM message_deliveries WHERE message_id=$1 AND recipient_id=$2`, beforeRejoin.ID, ids[1]).Scan(&oldStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `SELECT status FROM message_deliveries WHERE message_id=$1 AND recipient_id=$2`, afterRejoin.ID, ids[1]).Scan(&newStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `SELECT last_read_message_id FROM conversation_read_state WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1]).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "sent" || newStatus != "read" || cursor != afterRejoin.ID {
		t.Fatalf("rejoin visibility violated: old=%s new=%s cursor=%s", oldStatus, newStatus, cursor)
	}
}

func TestMessageInteractionsRespectVisibilityAndAreIdempotent(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 3)
	ctx := context.Background()
	cid := uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO conversations(id,kind,name,creator_id) VALUES($1,'group','interactions',$2)`, cid, ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO conversation_members(conversation_id,user_id,role,added_by) SELECT $1,x,CASE WHEN x=$2 THEN 'owner' ELSE 'member' END,$2 FROM unnest($3::uuid[]) x`, cid, ids[0], ids[:2]); err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	source := &model.Message{ID: uuid.NewString(), SenderID: ids[0], Body: "source", ConversationID: cid, CreatedAt: now, ModifiedAt: now}
	if _, err := r.CreateConversationMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	aggregates, changed, err := r.SetReaction(ctx, source.ID, ids[1], "👍", true)
	if err != nil || !changed || len(aggregates) != 1 || aggregates[0].Count != 1 || !aggregates[0].ReactedByMe {
		t.Fatalf("first reaction: %#v %t %v", aggregates, changed, err)
	}
	aggregates, changed, err = r.SetReaction(ctx, source.ID, ids[1], "👍", true)
	if err != nil || changed || aggregates[0].Count != 1 {
		t.Fatalf("duplicate reaction was not idempotent: %#v %t %v", aggregates, changed, err)
	}
	if _, _, err = r.SetReaction(ctx, source.ID, ids[2], "👍", true); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("outsider reaction: %v", err)
	}
	if _, err = db.Exec(ctx, `UPDATE conversation_members SET left_at=NOW() WHERE conversation_id=$1 AND user_id=$2`, cid, ids[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetVisibleMessage(ctx, source.ID, ids[1]); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("departed source visibility: %v", err)
	}
}

func TestMessageHistoryReturnsSafeReplyAndForwardPreviews(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	ctx := context.Background()
	c, err := NewConversationRepository(db).CreateOrGetDirect(ctx, ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	r := NewMessageRepositoryImpl(db)
	now := time.Now().UTC()
	source := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[1], Body: "original", ConversationID: c.ID, CreatedAt: now, ModifiedAt: now}
	if err = r.CreateMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	forward := &model.Message{ID: uuid.NewString(), SenderID: ids[1], ReceiverID: ids[0], Body: source.Body, ConversationID: c.ID, ReplyToMessageID: source.ID, ForwardedFromMessageID: source.ID, CreatedAt: now.Add(time.Second), ModifiedAt: now.Add(time.Second)}
	if err = r.CreateMessage(ctx, forward); err != nil {
		t.Fatal(err)
	}
	history, err := r.GetMessagesBetweenUsers(ctx, ids[1], ids[0], 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[1].ReplyTo == nil || history[1].ForwardedFrom == nil || history[1].ForwardedFrom.Content != "original" {
		t.Fatalf("missing interaction previews: %#v", history[1])
	}
}
