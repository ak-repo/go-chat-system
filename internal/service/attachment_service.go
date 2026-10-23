package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/platform/storage"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AttachmentRepository interface {
	Create(context.Context, *model.Attachment) error
	Get(context.Context, string) (*model.Attachment, error)
	Complete(context.Context, string, string) error
	Cancel(context.Context, string, string) error
	Claim(context.Context) (*model.Attachment, error)
	Finish(context.Context, *model.Attachment, string, string) error
	Publish(context.Context, *model.Attachment, func() error) error
	ClaimCleanup(context.Context) (*model.Attachment, error)
	Deleted(context.Context, string) error
	ExpiredStaging(context.Context) ([]string, error)
	StagingCleaned(context.Context, string) error
	ForMessage(context.Context, string) ([]model.Attachment, error)
}
type AttachmentService struct {
	repo     AttachmentRepository
	store    storage.Store
	messages *MessageServiceImpl
}

func NewAttachmentService(repo AttachmentRepository, store storage.Store, messages *MessageServiceImpl) *AttachmentService {
	return &AttachmentService{repo: repo, store: store, messages: messages}
}
func (s *AttachmentService) Enabled() bool { return s.store != nil }
func (s *MessageServiceImpl) AuthorizeImageUpload(ctx context.Context, actor, cid string) error {
	c, err := s.conversationRepo.Get(ctx, cid, actor)
	if err != nil {
		return errs.ErrNotMember
	}
	if c.Kind == "direct" {
		other := c.UserOneID
		if other == actor {
			other = c.UserTwoID
		}
		return s.authorizeDirect(ctx, actor, other)
	}
	return nil
}
func (s *AttachmentService) Initiate(ctx context.Context, owner, cid, name, mime string, size int64) (*model.Attachment, *storage.Upload, error) {
	if !s.Enabled() {
		return nil, nil, errs.ErrUnavailable
	}
	if uuid.Validate(cid) != nil || size <= 0 || size > MaxImageBytes || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") {
		return nil, nil, errs.ErrValidation
	}
	if err := s.messages.AuthorizeImageUpload(ctx, owner, cid); err != nil {
		return nil, nil, err
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if len(name) > 255 || strings.TrimSpace(name) == "" {
		return nil, nil, errs.ErrValidation
	}
	id := uuid.NewString()
	a := &model.Attachment{ID: id, OwnerID: owner, ConversationID: cid, Filename: name, DeclaredMIME: mime, DeclaredSize: size, StagingKey: "staging/" + id, FullKey: "media/" + id + "/full", ThumbnailKey: "media/" + id + "/thumbnail", State: "pending", UploadExpiresAt: time.Now().UTC().Add(10 * time.Minute)}
	upload, err := s.store.Upload(ctx, a.StagingKey, mime, size)
	if err != nil {
		return nil, nil, errs.Wrap("attachment.signUpload", err)
	}
	if err = s.repo.Create(ctx, a); err != nil {
		return nil, nil, err
	}
	return a, upload, nil
}
func (s *AttachmentService) Status(ctx context.Context, owner, id string) (*model.Attachment, error) {
	if uuid.Validate(id) != nil {
		return nil, errs.ErrValidation
	}
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.OwnerID != owner {
		return nil, errs.ErrNotFound
	}
	return a, nil
}
func (s *AttachmentService) Complete(ctx context.Context, owner, id string) (*model.Attachment, error) {
	if !s.Enabled() {
		return nil, errs.ErrUnavailable
	}
	if _, err := s.Status(ctx, owner, id); err != nil {
		return nil, err
	}
	if err := s.repo.Complete(ctx, id, owner); err != nil {
		return nil, err
	}
	return s.Status(ctx, owner, id)
}
func (s *AttachmentService) Cancel(ctx context.Context, owner, id string) error {
	if _, err := s.Status(ctx, owner, id); err != nil {
		return err
	}
	return s.repo.Cancel(ctx, id, owner)
}
func (s *AttachmentService) URL(ctx context.Context, owner, mid, id, variant string, download bool) (string, error) {
	if !s.Enabled() {
		return "", errs.ErrUnavailable
	}
	if uuid.Validate(mid) != nil || uuid.Validate(id) != nil || (variant != "full" && variant != "thumbnail") {
		return "", errs.ErrValidation
	}
	ir, ok := s.messages.messageRepo.(interface {
		GetVisibleMessage(context.Context, string, string) (*model.Message, error)
	})
	if !ok {
		return "", errs.ErrInternal
	}
	m, err := ir.GetVisibleMessage(ctx, mid, owner)
	if err != nil {
		return "", err
	}
	if err = s.messages.AuthorizeImageUpload(ctx, owner, m.ConversationID); err != nil {
		return "", err
	}
	items, err := s.repo.ForMessage(ctx, mid)
	if err != nil {
		return "", err
	}
	found := false
	for _, a := range items {
		if a.ID == id {
			found = true
		}
	}
	if !found {
		return "", errs.ErrNotFound
	}
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if a.State != "ready" {
		return "", errs.ErrNotFound
	}
	key := a.FullKey
	if variant == "thumbnail" {
		key = a.ThumbnailKey
	}
	return s.store.Download(ctx, key, a.MIMEType, download)
}

// Run performs bounded, durable work outside HTTP/WS deadlines. A single decoder
// limits peak memory; database leases allow recovery after process termination.
func (s *AttachmentService) Run(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}
func (s *AttachmentService) tick(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel()
	a, err := s.repo.Claim(ctx)
	if err == nil {
		s.process(ctx, a)
	} else if !errors.Is(err, errs.ErrNotFound) && ctx.Err() == nil {
		logger.L().Error("claim image processing", zap.Error(err))
	}
	cancel()
	ctx, cleanupCancel := context.WithTimeout(parent, time.Minute)
	defer cleanupCancel()
	a, err = s.repo.ClaimCleanup(ctx)
	if err == nil {
		ok := true
		for _, key := range []string{a.StagingKey, a.FullKey, a.ThumbnailKey} {
			if err = s.store.Delete(ctx, key); err != nil {
				ok = false
				logger.L().Error("delete image object", zap.String("attachment_id", a.ID), zap.Error(err))
			}
		}
		if ok {
			if err = s.repo.Deleted(ctx, a.ID); err != nil {
				logger.L().Error("finish image cleanup", zap.Error(err))
			}
		}
	} else if !errors.Is(err, errs.ErrNotFound) && ctx.Err() == nil {
		logger.L().Error("claim image cleanup", zap.Error(err))
	}
	keys, err := s.repo.ExpiredStaging(ctx)
	if err != nil {
		return
	}
	for _, key := range keys {
		if s.store.Delete(ctx, key) == nil {
			_ = s.repo.StagingCleaned(ctx, key)
		}
	}
}
func (s *AttachmentService) process(ctx context.Context, a *model.Attachment) {
	fail := func(reason string, retry bool) {
		state := "failed"
		if retry && a.Attempts < 3 {
			state = "queued"
		}
		if err := s.repo.Finish(ctx, a, state, reason); err != nil {
			logger.L().Error("finish image processing", zap.String("attachment_id", a.ID), zap.Error(err))
		}
	}
	data, err := s.store.Read(ctx, a.StagingKey, MaxImageBytes)
	if err != nil {
		fail("Could not read upload; try uploading again", true)
		return
	}
	if int64(len(data)) != a.DeclaredSize {
		fail("Uploaded size does not match", false)
		return
	}
	image, err := ProcessImage(data, a.DeclaredMIME)
	if err != nil {
		fail(err.Error(), false)
		return
	}
	a.MIMEType = image.MIME
	a.Size = int64(len(image.Full))
	a.Width = image.Width
	a.Height = image.Height
	err = s.repo.Publish(ctx, a, func() error {
		if err := s.store.Put(ctx, a.FullKey, image.MIME, image.Full); err != nil {
			return err
		}
		return s.store.Put(ctx, a.ThumbnailKey, image.MIME, image.Thumbnail)
	})
	if err != nil {
		if !errors.Is(err, errs.ErrConflict) {
			fail("Could not store processed image", true)
		}
		return
	}
	// Wait for the upload policy to expire before permanently marking staging
	// cleaned: the client can replay its POST until that point.
	_ = s.store.Delete(ctx, a.StagingKey)
}
