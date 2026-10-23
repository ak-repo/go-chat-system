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

func TestImageAttachmentLifecycleAndForwardRetention(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	users := integrationUsers(t, db, 3)
	c, err := NewConversationRepository(db).CreateOrGetDirect(ctx, users[0], users[1])
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewConversationRepository(db).CreateOrGetDirect(ctx, users[1], users[2])
	if err != nil {
		t.Fatal(err)
	}
	ar := NewAttachmentRepository(db)
	mr := NewMessageRepositoryImpl(db)
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: users[0], ConversationID: c.ID, Filename: "photo.png", DeclaredMIME: "image/png", DeclaredSize: 10, StagingKey: "staging/" + id, FullKey: "full/" + id, ThumbnailKey: "thumb/" + id, UploadExpiresAt: time.Now().Add(-time.Minute)}
	if err = ar.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = ar.Complete(ctx, id, users[2]); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("nonowner completion: %v", err)
	}
	if err = ar.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	if err = ar.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	claimed, err := ar.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Attempts != 1 {
		t.Fatal("attempt not recorded")
	}
	// An expired lease is recoverable and the previous attempt cannot finalize it.
	if _, err = db.Exec(ctx, `UPDATE attachments SET lease_until=NOW()-INTERVAL '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	recovered, err := ar.Claim(ctx)
	if err != nil || recovered.Attempts != 2 {
		t.Fatalf("recovery %#v %v", recovered, err)
	}
	if err = ar.Finish(ctx, claimed, "ready", ""); !errors.Is(err, errs.ErrConflict) {
		t.Fatal(err)
	}
	got, err := ar.Get(ctx, id)
	if err != nil || got.State != "processing" {
		t.Fatalf("stale worker finalized %#v %v", got, err)
	}
	recovered.MIMEType = "image/jpeg"
	recovered.Size = 8
	recovered.Width = 10
	recovered.Height = 5
	if err = ar.Finish(ctx, recovered, "ready", ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	makeMessage := func(sender, cid string) *model.Message {
		return &model.Message{ID: uuid.NewString(), SenderID: sender, ConversationID: cid, AttachmentIDs: []string{id}, CreatedAt: now, ModifiedAt: now, ClientMessageID: uuid.NewString()}
	}
	stolen := makeMessage(users[1], c.ID)
	if _, err = mr.CreateConversationMessage(ctx, stolen); !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("stolen attachment: %v", err)
	}
	source := makeMessage(users[0], c.ID)
	if _, err = mr.CreateConversationMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	if len(source.Attachments) != 1 || source.MessageType != "image" || len(source.CreatedNotifications) != 1 || source.CreatedNotifications[0].Payload["content"] != "Image" {
		t.Fatalf("canonical image: %#v", source)
	}
	retry, err := mr.GetByClientMessageID(ctx, source.ClientMessageID, users[0], c.ID)
	if err != nil || len(retry.Attachments) != 1 {
		t.Fatalf("retry metadata %#v %v", retry, err)
	}
	if _, err = mr.CreateConversationMessage(ctx, makeMessage(users[0], c.ID)); !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("attachment reused: %v", err)
	}
	forward := makeMessage(users[1], other.ID)
	forward.ForwardedFromMessageID = source.ID
	if _, err = mr.CreateConversationMessage(ctx, forward); err != nil {
		t.Fatal(err)
	}
	if err = ar.Cancel(ctx, id, users[0]); !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("cancel linked: %v", err)
	}
	if err = mr.DeleteMessage(ctx, source.ID, users[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = ar.ClaimCleanup(ctx); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("deleted live forward: %v", err)
	}
	if m, err := mr.GetVisibleMessage(ctx, forward.ID, users[2]); err != nil || len(m.Attachments) != 1 {
		t.Fatalf("forward broken %#v %v", m, err)
	}
	if err = mr.DeleteMessage(ctx, forward.ID, users[1]); err != nil {
		t.Fatal(err)
	}
	cleanup, err := ar.ClaimCleanup(ctx)
	if err != nil || cleanup.ID != id {
		t.Fatalf("cleanup %#v %v", cleanup, err)
	}
	if err = ar.Deleted(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestAttachmentPublishSerializesCancellation(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	users := integrationUsers(t, db, 2)
	c, err := NewConversationRepository(db).CreateOrGetDirect(ctx, users[0], users[1])
	if err != nil {
		t.Fatal(err)
	}
	r := NewAttachmentRepository(db)
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: users[0], ConversationID: c.ID, Filename: "test.png", DeclaredMIME: "image/png", DeclaredSize: 1, StagingKey: id + "/staging", FullKey: id + "/full", ThumbnailKey: id + "/thumb", UploadExpiresAt: time.Now().Add(-time.Minute)}
	if err = r.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = r.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	a, err = r.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	published := make(chan error, 1)
	go func() { published <- r.Publish(ctx, a, func() error { close(entered); <-release; return nil }) }()
	<-entered
	cancelled := make(chan error, 1)
	go func() { cancelled <- r.Cancel(ctx, id, users[0]) }()
	select {
	case err := <-cancelled:
		close(release)
		t.Fatalf("cancel bypassed publication lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err = <-published; err != nil {
		t.Fatal(err)
	}
	if err = <-cancelled; err != nil {
		t.Fatal(err)
	}
	called := false
	if err = r.Publish(ctx, a, func() error { called = true; return nil }); !errors.Is(err, errs.ErrConflict) || called {
		t.Fatalf("stale publication: %v, wrote=%v", err, called)
	}
	cleanup, err := r.ClaimCleanup(ctx)
	if err != nil || cleanup.ID != id {
		t.Fatalf("cleanup: %#v %v", cleanup, err)
	}
}

func TestAttachmentExhaustedLeaseAndPartialWriteRecovery(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	users := integrationUsers(t, db, 2)
	c, err := NewConversationRepository(db).CreateOrGetDirect(ctx, users[0], users[1])
	if err != nil {
		t.Fatal(err)
	}
	r := NewAttachmentRepository(db)
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: users[0], ConversationID: c.ID, Filename: "test.png", DeclaredMIME: "image/png", DeclaredSize: 1, StagingKey: id + "/staging", FullKey: id + "/full", ThumbnailKey: id + "/thumb", UploadExpiresAt: time.Now().Add(-time.Minute)}
	if err = r.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = r.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	a, err = r.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	failed := errors.New("thumbnail storage unavailable")
	if err = r.Publish(ctx, a, func() error { return failed }); !errors.Is(err, failed) {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, id)
	if err != nil || got.State != "processing" {
		t.Fatalf("published partial media: %#v %v", got, err)
	}
	if _, err = db.Exec(ctx, `UPDATE attachments SET attempts=3,lease_until=NOW()-INTERVAL '1 second',created_at=NOW()-INTERVAL '25 hours' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Claim(ctx); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("claimed exhausted lease: %v", err)
	}
	got, err = r.Get(ctx, id)
	if err != nil || got.State != "failed" {
		t.Fatalf("not retired: %#v %v", got, err)
	}
	if _, err = r.ClaimCleanup(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentImageLinksAndClientIDs(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	users := integrationUsers(t, db, 2)
	c, err := NewConversationRepository(db).CreateOrGetDirect(ctx, users[0], users[1])
	if err != nil {
		t.Fatal(err)
	}
	r := NewAttachmentRepository(db)
	mr := NewMessageRepositoryImpl(db)
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: users[0], ConversationID: c.ID, Filename: "test.png", DeclaredMIME: "image/png", DeclaredSize: 1, StagingKey: id + "/staging", FullKey: id + "/full", ThumbnailKey: id + "/thumb", UploadExpiresAt: time.Now().Add(-time.Minute)}
	if err = r.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = r.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	a, err = r.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Publish(ctx, a, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	clientID := uuid.NewString()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			now := time.Now()
			m := &model.Message{ID: uuid.NewString(), SenderID: users[0], ConversationID: c.ID, ClientMessageID: clientID, AttachmentIDs: []string{id}, CreatedAt: now, ModifiedAt: now}
			_, err := mr.CreateConversationMessage(ctx, m)
			results <- err
		}()
	}
	close(start)
	success := 0
	for range 2 {
		if err = <-results; err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful inserts=%d", success)
	}
	var links, notifications int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM message_attachments WHERE attachment_id=$1`, id).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE message_id IN (SELECT message_id FROM message_attachments WHERE attachment_id=$1)`, id).Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if links != 1 || notifications != 1 {
		t.Fatalf("links=%d notifications=%d", links, notifications)
	}
	old, err := mr.GetByClientMessageID(ctx, clientID, users[0], c.ID)
	if err != nil || old == nil || len(old.Attachments) != 1 {
		t.Fatalf("retry=%#v %v", old, err)
	}
}

func TestImageGroupHistoryAndMembershipBoundaries(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	users := integrationUsers(t, db, 3)
	friends := NewFriendRepositoryImpl(db)
	for _, member := range users[1:] {
		if err := friends.CreateFriendship(ctx, users[0], member); err != nil {
			t.Fatal(err)
		}
	}
	conversations := NewConversationRepository(db)
	group, err := conversations.CreateGroup(ctx, users[0], "images", nil, []string{users[1]})
	if err != nil {
		t.Fatal(err)
	}
	target, err := conversations.CreateOrGetDirect(ctx, users[0], users[2])
	if err != nil {
		t.Fatal(err)
	}
	r := NewAttachmentRepository(db)
	mr := NewMessageRepositoryImpl(db)
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: users[0], ConversationID: group.ID, Filename: "group.png", DeclaredMIME: "image/png", DeclaredSize: 1, StagingKey: id + "/staging", FullKey: id + "/full", ThumbnailKey: id + "/thumb", UploadExpiresAt: time.Now().Add(-time.Minute)}
	if err = r.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = r.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	a, err = r.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Publish(ctx, a, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	source := &model.Message{ID: uuid.NewString(), SenderID: users[0], ConversationID: group.ID, AttachmentIDs: []string{id}, CreatedAt: now, ModifiedAt: now}
	if _, err = mr.CreateConversationMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	if _, err = mr.GetVisibleMessage(ctx, source.ID, users[2]); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("outsider sees image: %v", err)
	}
	if _, err = conversations.AddGroupMembers(ctx, group.ID, users[0], []string{users[2]}); err != nil {
		t.Fatal(err)
	}
	if _, err = mr.GetVisibleMessage(ctx, source.ID, users[2]); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("new member sees old image: %v", err)
	}
	forward := &model.Message{ID: uuid.NewString(), SenderID: users[2], ConversationID: target.ID, AttachmentIDs: []string{id}, ForwardedFromMessageID: source.ID, CreatedAt: time.Now(), ModifiedAt: time.Now()}
	if _, err = mr.CreateConversationMessage(ctx, forward); !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("new member forwarded invisible image: %v", err)
	}
	if _, err = mr.GetVisibleMessage(ctx, source.ID, users[1]); err != nil {
		t.Fatal(err)
	}
	if err = conversations.RemoveGroupMember(ctx, group.ID, users[0], users[1]); err != nil {
		t.Fatal(err)
	}
	if _, err = mr.GetVisibleMessage(ctx, source.ID, users[1]); !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("removed member sees image: %v", err)
	}
}

func TestImageForwardRacesFinalSourceDeletionAndCleanup(t *testing.T) {
	db := integrationDB(t)
	ctx := context.Background()
	users := integrationUsers(t, db, 3)
	conversations := NewConversationRepository(db)
	sourceConversation, err := conversations.CreateOrGetDirect(ctx, users[0], users[1])
	if err != nil {
		t.Fatal(err)
	}
	targetConversation, err := conversations.CreateOrGetDirect(ctx, users[1], users[2])
	if err != nil {
		t.Fatal(err)
	}
	attachments := NewAttachmentRepository(db)
	messages := NewMessageRepositoryImpl(db)
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: users[0], ConversationID: sourceConversation.ID, Filename: "race.png", DeclaredMIME: "image/png", DeclaredSize: 1, StagingKey: id + "/staging", FullKey: id + "/full", ThumbnailKey: id + "/thumb", UploadExpiresAt: time.Now().Add(-time.Minute)}
	if err = attachments.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = attachments.Complete(ctx, id, users[0]); err != nil {
		t.Fatal(err)
	}
	a, err = attachments.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = attachments.Publish(ctx, a, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	source := &model.Message{ID: uuid.NewString(), SenderID: users[0], ConversationID: sourceConversation.ID, AttachmentIDs: []string{id}, CreatedAt: now, ModifiedAt: now}
	if _, err = messages.CreateConversationMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	forward := &model.Message{ID: uuid.NewString(), SenderID: users[1], ConversationID: targetConversation.ID, AttachmentIDs: []string{id}, ForwardedFromMessageID: source.ID, CreatedAt: time.Now(), ModifiedAt: time.Now()}
	start := make(chan struct{})
	forwarded := make(chan error, 1)
	deleted := make(chan error, 1)
	cleaned := make(chan error, 1)
	go func() { <-start; _, err := messages.CreateConversationMessage(ctx, forward); forwarded <- err }()
	go func() { <-start; deleted <- messages.DeleteMessage(ctx, source.ID, users[0]) }()
	go func() { <-start; _, err := attachments.ClaimCleanup(ctx); cleaned <- err }()
	close(start)
	forwardErr, deleteErr, cleanupErr := <-forwarded, <-deleted, <-cleaned
	if deleteErr != nil {
		t.Fatal(deleteErr)
	}
	if cleanupErr != nil && !errors.Is(cleanupErr, errs.ErrNotFound) {
		t.Fatal(cleanupErr)
	}
	if forwardErr == nil {
		a, err = attachments.Get(ctx, id)
		if err != nil || a.State != "ready" {
			t.Fatalf("live forward lost media: %#v %v", a, err)
		}
		if _, err = attachments.ClaimCleanup(ctx); !errors.Is(err, errs.ErrNotFound) {
			t.Fatalf("cleaned live forward: %v", err)
		}
	} else if !errors.Is(forwardErr, errs.ErrForbidden) && !errors.Is(forwardErr, errs.ErrConflict) {
		t.Fatal(forwardErr)
	}
}
