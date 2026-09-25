//go:build integration

package repository

import (
	"context"
	"testing"
	"time"
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

func TestConversationRepositoryPreferencesAreMemberScoped(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	r := NewConversationRepository(db)
	c, err := r.CreateOrGetDirect(context.Background(), ids[0], ids[1])
	if err != nil {
		t.Fatal(err)
	}
	archived, pinned := true, true
	muteUntil := time.Now().UTC().Add(time.Hour)
	if err := r.UpdatePreferences(context.Background(), c.ID, ids[0], &archived, &pinned, &muteUntil); err != nil {
		t.Fatal(err)
	}
	active, err := r.ListWithPreferences(context.Background(), ids[0], 10, 0, false)
	if err != nil || len(active) != 0 {
		t.Fatalf("archived conversation listed as active: %#v, %v", active, err)
	}
	archivedList, err := r.ListWithPreferences(context.Background(), ids[0], 10, 0, true)
	if err != nil || len(archivedList) != 1 || !archivedList[0].Archived || !archivedList[0].Pinned || !archivedList[0].Muted {
		t.Fatalf("preferences not listed: %#v, %v", archivedList, err)
	}
	other, err := r.ListWithPreferences(context.Background(), ids[1], 10, 0, false)
	if err != nil || len(other) != 1 || other[0].Archived || other[0].Pinned || other[0].Muted {
		t.Fatalf("state leaked to other member: %#v, %v", other, err)
	}
	if err := r.Hide(context.Background(), c.ID, ids[0]); err != nil {
		t.Fatal(err)
	}
	active, err = r.ListWithPreferences(context.Background(), ids[0], 10, 0, true)
	if err != nil || len(active) != 0 {
		t.Fatalf("hidden conversation remained visible: %#v, %v", active, err)
	}
}
