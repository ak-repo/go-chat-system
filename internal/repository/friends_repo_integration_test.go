//go:build integration

package repository

import (
	"context"
	"testing"
)

func TestFriendRepositoryCreatesMutualFriendship(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	r := NewFriendRepositoryImpl(db)
	if err := r.CreateFriendship(context.Background(), ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{ids[0], ids[1]}, {ids[1], ids[0]}} {
		ok, err := r.AreFriends(context.Background(), pair[0], pair[1])
		if err != nil || !ok {
			t.Fatalf("friendship %v: %v, %v", pair, ok, err)
		}
	}
	if got, err := r.ListFriends(context.Background(), ids[0], 20, 0); err != nil || len(got) != 1 {
		t.Fatalf("list friends: %d, %v", len(got), err)
	}
}
