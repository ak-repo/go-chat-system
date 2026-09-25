//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
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
	m := &model.Message{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[1], Body: "hello", ConversationID: conversation.ID, ClientMessageID: uuid.NewString(), CreatedAt: now, ModifiedAt: now}
	if err := r.CreateMessage(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetMessage(context.Background(), m.ID)
	if err != nil || got.Body != "hello" {
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
