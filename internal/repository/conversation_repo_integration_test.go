//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
)

func TestGroupInviteLifecycleAndIdempotentAcceptance(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 3)
	r := NewConversationRepository(db)
	c, err := r.CreateGroup(context.Background(), ids[0], "Invites", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := HashGroupInviteToken("integration-secret")
	one := 1
	invite, err := r.CreateGroupInvite(context.Background(), c.ID, ids[0], hash, time.Now().Add(time.Hour), &one)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptGroupInvite(context.Background(), ids[1], hash); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptGroupInvite(context.Background(), ids[1], hash); err != nil {
		t.Fatalf("active member should accept an exhausted invite idempotently: %v", err)
	}
	var uses int
	if err = db.QueryRow(context.Background(), `SELECT use_count FROM group_invites WHERE id=$1`, invite.ID).Scan(&uses); err != nil || uses != 1 {
		t.Fatalf("use count=%d err=%v", uses, err)
	}
	secondHash := HashGroupInviteToken("second-secret")
	second, err := r.CreateGroupInvite(context.Background(), c.ID, ids[0], secondHash, time.Now().Add(time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.AcceptGroupInvite(context.Background(), ids[1], secondHash); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(context.Background(), `SELECT use_count FROM group_invites WHERE id=$1`, second.ID).Scan(&uses); err != nil || uses != 0 {
		t.Fatalf("active member consumed invite: %d %v", uses, err)
	}
	if err = r.RevokeGroupInvite(context.Background(), c.ID, ids[0], second.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := r.ListGroupInvites(context.Background(), c.ID, ids[0])
	if err != nil || len(listed) != 2 || listed[0].RevokedAt == nil {
		t.Fatalf("list/revoke: %#v %v", listed, err)
	}
}

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

func TestConversationRepositoryRejectsDirectSelfConversation(t *testing.T) {
	db := integrationDB(t)
	id := integrationUsers(t, db, 1)[0]
	if _, err := NewConversationRepository(db).CreateOrGetDirect(context.Background(), id, id); !errors.Is(err, errs.ErrSelfAction) {
		t.Fatalf("expected self conversation rejection, got %v", err)
	}
}

func TestPresenceAudienceFiltersBlocksAndIncludesSharedGroups(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 4)
	if _, err := db.Exec(context.Background(), `INSERT INTO friends(user_id,friend_id) VALUES($1,$2),($2,$1),($1,$3),($3,$1),($1,$4),($4,$1)`, ids[0], ids[1], ids[2], ids[3]); err != nil {
		t.Fatal(err)
	}
	r := NewConversationRepository(db)
	if _, err := r.CreateGroup(context.Background(), ids[0], "Presence", nil, []string{ids[2], ids[3]}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO blocks(blocker_id,blocked_id) VALUES($1,$2)`, ids[3], ids[0]); err != nil {
		t.Fatal(err)
	}
	audience, err := r.PresenceAudience(context.Background(), ids[0])
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{ids[1]: true, ids[2]: true}
	if len(audience) != len(want) {
		t.Fatalf("unexpected audience: %v", audience)
	}
	for _, id := range audience {
		if !want[id] {
			t.Fatalf("unauthorized audience member: %s", id)
		}
	}
}

func TestGroupConversationAdministrationAndRejoin(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 4)
	for _, member := range ids[1:] {
		if _, err := db.Exec(context.Background(), `INSERT INTO friends(user_id,friend_id) VALUES($1,$2),($2,$1)`, ids[0], member); err != nil {
			t.Fatal(err)
		}
	}
	r := NewConversationRepository(db)
	c, err := r.CreateGroup(context.Background(), ids[0], "Core team", nil, ids[1:])
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != "group" || c.MemberCount != 4 || c.CurrentRole != model.GroupRoleOwner || c.Capabilities == nil || !c.Capabilities.ManageRoles {
		t.Fatalf("unexpected group response: %#v", c)
	}
	listed, err := r.List(context.Background(), ids[1], 10, 0)
	if err != nil || len(listed) != 1 || listed[0].ID != c.ID || listed[0].CurrentRole != model.GroupRoleMember {
		t.Fatalf("group was not included in shared conversation list: %#v, %v", listed, err)
	}
	if err = r.UpdateGroupMemberRole(context.Background(), c.ID, ids[0], ids[1], model.GroupRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err = r.RemoveGroupMember(context.Background(), c.ID, ids[1], ids[2]); err != nil {
		t.Fatalf("admin should remove ordinary member: %v", err)
	}
	if err = r.RemoveGroupMember(context.Background(), c.ID, ids[1], ids[0]); !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("admin removed owner: %v", err)
	}
	var oldJoined time.Time
	if err = db.QueryRow(context.Background(), `SELECT joined_at FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, c.ID, ids[2]).Scan(&oldJoined); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err = r.AddGroupMembers(context.Background(), c.ID, ids[0], []string{ids[2]}); err != nil {
		t.Fatal(err)
	}
	var role string
	var joined time.Time
	if err = db.QueryRow(context.Background(), `SELECT role,joined_at FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, c.ID, ids[2]).Scan(&role, &joined); err != nil {
		t.Fatal(err)
	}
	if role != model.GroupRoleMember || !joined.After(oldJoined) {
		t.Fatalf("rejoin did not reset membership: role=%s old=%s joined=%s", role, oldJoined, joined)
	}
}

func TestGroupConversationProtectsFinalOwner(t *testing.T) {
	db := integrationDB(t)
	ids := integrationUsers(t, db, 2)
	if _, err := db.Exec(context.Background(), `INSERT INTO friends(user_id,friend_id) VALUES($1,$2),($2,$1)`, ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	r := NewConversationRepository(db)
	c, err := r.CreateGroup(context.Background(), ids[0], "Owners", nil, []string{ids[1]})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.LeaveGroup(context.Background(), c.ID, ids[0]); !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("final owner left: %v", err)
	}
	if err = r.UpdateGroupMemberRole(context.Background(), c.ID, ids[0], ids[0], model.GroupRoleMember); !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("final owner demoted: %v", err)
	}
	if err = r.UpdateGroupMemberRole(context.Background(), c.ID, ids[0], ids[1], model.GroupRoleOwner); err != nil {
		t.Fatal(err)
	}
	if err = r.LeaveGroup(context.Background(), c.ID, ids[0]); err != nil {
		t.Fatalf("owner could not leave after transfer: %v", err)
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
