//go:build integration

package repository

import (
	"context"
	"testing"
)

func TestConversationRepositoryCreatesCanonicalDirectConversation(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	r := NewConversationRepository(db)
	one, err := r.CreateOrGetDirect(context.Background(), ids[1], ids[0])
	if err != nil {
		t.Fatal(err)
	}
	two, err := r.CreateOrGetDirect(context.Background(), ids[0], ids[1])
	if err != nil || one.ID != two.ID {
		t.Fatalf("conversation should be idempotent: %#v, %#v, %v", one, two, err)
	}
	for _, uid := range ids {
		ok, err := r.IsMember(context.Background(), one.ID, uid)
		if err != nil || !ok {
			t.Fatalf("member %s: %v, %v", uid, ok, err)
		}
	}
	if got, err := r.List(context.Background(), ids[0], 10, 0); err != nil || len(got) != 1 {
		t.Fatalf("list conversations: %d, %v", len(got), err)
	}
}
