//go:build integration

package repository

import (
	"context"
	"testing"
)

func TestBlockRepositoryRemovesFriendshipAndBlocksRequests(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	friends := NewFriendRepositoryImpl(db)
	if err := friends.CreateFriendship(context.Background(), ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	r := BlockRepositoryInit(db)
	if err := r.BlockUser(context.Background(), ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	blocked, err := r.IsBlocked(context.Background(), ids[0], ids[1])
	if err != nil || !blocked {
		t.Fatalf("expected blocked relationship: %v, %v", blocked, err)
	}
	friendsOK, err := friends.AreFriends(context.Background(), ids[0], ids[1])
	if err != nil || friendsOK {
		t.Fatalf("block should remove friendship: %v, %v", friendsOK, err)
	}
	if err := r.UnblockUser(context.Background(), ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	blocked, err = r.IsBlocked(context.Background(), ids[0], ids[1])
	if err != nil || blocked {
		t.Fatalf("expected unblock: %v, %v", blocked, err)
	}
}
