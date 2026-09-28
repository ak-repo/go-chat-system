package service

import (
	"context"

	"github.com/ak-repo/go-chat-system/internal/repository"
)

type PresenceService interface {
	Audience(context.Context, string) ([]string, error)
}

type PresenceServiceImpl struct{ repo repository.PresenceRepository }

func NewPresenceService(repo repository.PresenceRepository) *PresenceServiceImpl {
	return &PresenceServiceImpl{repo: repo}
}

func (s *PresenceServiceImpl) Audience(ctx context.Context, userID string) ([]string, error) {
	return s.repo.PresenceAudience(ctx, userID)
}
