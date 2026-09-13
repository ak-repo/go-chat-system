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
