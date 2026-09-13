//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
)

func TestFriendRequestRepositoryAcceptsAndCreatesFriendship(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	r := FriendRequestRepositoryInit(db)
	req := &model.FriendRequest{ID: uuid.NewString(), SenderID: ids[0], ReceiverID: ids[1], Status: model.FriendPending, CreatedAt: time.Now().UTC()}
	if err := r.CreateRequest(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got, err := r.GetPendingRequest(context.Background(), ids[0], ids[1]); err != nil || got == nil {
		t.Fatalf("pending request: %#v, %v", got, err)
	}
	if err := r.AcceptRequest(context.Background(), req.ID, ids[1]); err != nil {
		t.Fatal(err)
	}
	friends := NewFriendRepositoryImpl(db)
	ok, err := friends.AreFriends(context.Background(), ids[0], ids[1])
	if err != nil || !ok {
		t.Fatalf("accepted request should create friendship: %v, %v", ok, err)
	}
	if err := r.CancelRequest(context.Background(), req.ID, ids[0]); err == nil || !errs.Is(err, errs.ErrRequestNotFound) {
		t.Fatalf("expected missing request after acceptance, got %v", err)
	}
}
