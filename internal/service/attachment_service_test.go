package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/platform/storage"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
)

type attachmentRepoFake struct {
	AttachmentRepository
	item       *model.Attachment
	linked     []model.Attachment
	publishErr error
	final      string
}

func (f *attachmentRepoFake) Get(context.Context, string) (*model.Attachment, error) {
	return f.item, nil
}
func (f *attachmentRepoFake) ForMessage(context.Context, string) ([]model.Attachment, error) {
	return f.linked, nil
}
func (f *attachmentRepoFake) Finish(_ context.Context, _ *model.Attachment, state, _ string) error {
	f.final = state
	return nil
}
func (f *attachmentRepoFake) Publish(_ context.Context, _ *model.Attachment, write func() error) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	return write()
}

type imageStoreFake struct {
	storage.Store
	data          []byte
	puts, deletes int
	failPut       int
	downloads     int
}

func (f *imageStoreFake) Read(context.Context, string, int64) ([]byte, error) { return f.data, nil }
func (f *imageStoreFake) Put(context.Context, string, string, []byte) error {
	f.puts++
	if f.puts == f.failPut {
		return errors.New("storage failed")
	}
	return nil
}
func (f *imageStoreFake) Delete(context.Context, string) error { f.deletes++; return nil }
func (f *imageStoreFake) Download(context.Context, string, string, bool) (string, error) {
	f.downloads++
	return "signed", nil
}

func TestAttachmentProcessingFailureAndStaleAttempt(t *testing.T) {
	for _, tc := range []struct {
		name          string
		publishErr    error
		failPut       int
		final         string
		puts, deletes int
	}{
		{name: "ready", puts: 2, deletes: 1},
		{name: "stale", publishErr: errs.ErrConflict},
		{name: "partial upload", failPut: 2, final: "queued", puts: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := testPNG(t, 4, 2, false)
			repo := &attachmentRepoFake{publishErr: tc.publishErr}
			store := &imageStoreFake{data: data, failPut: tc.failPut}
			s := NewAttachmentService(repo, store, nil)
			s.process(context.Background(), &model.Attachment{ID: uuid.NewString(), DeclaredSize: int64(len(data)), DeclaredMIME: "image/png", Attempts: 1})
			if repo.final != tc.final || store.puts != tc.puts || store.deletes != tc.deletes {
				t.Fatalf("state=%s puts=%d deletes=%d", repo.final, store.puts, store.deletes)
			}
		})
	}
}
func TestAttachmentStatusOwnerAndDisabledUpload(t *testing.T) {
	id := uuid.NewString()
	repo := &attachmentRepoFake{item: &model.Attachment{ID: id, OwnerID: "owner"}}
	s := NewAttachmentService(repo, nil, nil)
	if _, _, err := s.Initiate(context.Background(), "owner", uuid.NewString(), "a.png", "image/png", 10); !errors.Is(err, errs.ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.Status(context.Background(), "other", id); !errors.Is(err, errs.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Status(context.Background(), "owner", "invalid"); !errors.Is(err, errs.ErrValidation) {
		t.Fatal(err)
	}
	if _, err := s.Status(context.Background(), "owner", id); err != nil {
		t.Fatal(err)
	}
}

func TestImageURLRequiresVisibleLinkedMessage(t *testing.T) {
	id, mid, cid := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name       string
		visibleErr error
		linked     bool
		state      string
		want       error
	}{
		{name: "allowed", linked: true, state: "ready"},
		{name: "invisible", visibleErr: errs.ErrNotFound, want: errs.ErrNotFound},
		{name: "unlinked", state: "ready", want: errs.ErrNotFound},
		{name: "deleted", linked: true, state: "deleted", want: errs.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mr := &fakeMessageRepo{target: &model.Message{ID: mid, ConversationID: cid}, err: tc.visibleErr}
			messages := NewMessageServiceImpl(mr, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})
			messages.SetConversationRepository(&fakeConversationRepo{conversation: &model.Conversation{ID: cid, Kind: "group"}})
			repo := &attachmentRepoFake{item: &model.Attachment{ID: id, State: tc.state}}
			if tc.linked {
				repo.linked = []model.Attachment{{ID: id}}
			}
			store := &imageStoreFake{}
			_, err := NewAttachmentService(repo, store, messages).URL(context.Background(), "actor", mid, id, "full", false)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if tc.want != nil && store.downloads != 0 {
				t.Fatal("signed unauthorized media")
			}
		})
	}
}
func TestImageUploadAuthorizationAndValidation(t *testing.T) {
	cid := uuid.NewString()
	messages := NewMessageServiceImpl(&fakeMessageRepo{}, fakeFriendRepo{areFriends: true}, fakeBlockRepo{blocked: true})
	messages.SetConversationRepository(&fakeConversationRepo{conversation: &model.Conversation{ID: cid, Kind: "direct", UserOneID: "actor", UserTwoID: "other"}})
	s := NewAttachmentService(&attachmentRepoFake{}, &imageStoreFake{}, messages)
	if _, _, err := s.Initiate(context.Background(), "actor", cid, "a.png", "image/png", 10); !errors.Is(err, errs.ErrBlockedRelationship) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mime string
		size int64
	}{{"text/html", 10}, {"image/png", 0}, {"image/png", MaxImageBytes + 1}} {
		if _, _, err := s.Initiate(context.Background(), "actor", cid, "a.png", tc.mime, tc.size); !errors.Is(err, errs.ErrValidation) {
			t.Fatal(err)
		}
	}
}
