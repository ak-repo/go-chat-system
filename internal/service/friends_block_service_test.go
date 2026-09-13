package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
)

type fakeFriendsRepo struct {
	userID        string
	limit, offset int
	err           error
}

func (f *fakeFriendsRepo) CreateFriendship(context.Context, string, string) error { return nil }
func (f *fakeFriendsRepo) AreFriends(context.Context, string, string) (bool, error) {
	return false, nil
}
func (f *fakeFriendsRepo) ListFriends(_ context.Context, id string, limit, offset int) (model.FriendsDTO, error) {
	f.userID, f.limit, f.offset = id, limit, offset
	return nil, f.err
}

func TestListFriendsUsesAuthenticatedIdentityAndPagination(t *testing.T) {
	repo := &fakeFriendsRepo{}
	status, response, err := NewFriendServiceImpl(repo).ListFriends(httptest.NewRecorder(), authenticatedRequest(http.MethodGet, "", "user-1"))
	if status != http.StatusOK || response == nil || err != nil || repo.userID != "user-1" || repo.limit != 20 || repo.offset != 0 {
		t.Fatalf("unexpected friend list result: %d, %v, %#v", status, err, repo)
	}
}

func TestListFriendsReturnsDependencyError(t *testing.T) {
	repo := &fakeFriendsRepo{err: errors.New("database unavailable")}
	status, _, err := NewFriendServiceImpl(repo).ListFriends(httptest.NewRecorder(), authenticatedRequest(http.MethodGet, "?limit=bad", "user-1"))
	if status != http.StatusInternalServerError || err == nil {
		t.Fatalf("expected dependency error, got %d, %v", status, err)
	}
}

type fakeBlockRepoService struct {
	blocker, target string
	err             error
}

func (f *fakeBlockRepoService) BlockUser(_ context.Context, blocker, target string) error {
	f.blocker, f.target = blocker, target
	return f.err
}
func (f *fakeBlockRepoService) UnblockUser(_ context.Context, blocker, target string) error {
	f.blocker, f.target = blocker, target
	return f.err
}
func (f *fakeBlockRepoService) IsBlocked(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestBlockUserRejectsSelfAndUsesAuthenticatedBlocker(t *testing.T) {
	repo := &fakeBlockRepoService{}
	service := BlockServiceInit(repo)
	status, _, err := service.BlockUser(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"target":"user-1"}`, "user-1"))
	if status != http.StatusConflict || !errors.Is(err, errs.ErrSelfAction) {
		t.Fatalf("expected self-action conflict, got %d, %v", status, err)
	}
	status, _, err = service.BlockUser(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"target":"user-2"}`, "user-1"))
	if status != http.StatusOK || err != nil || repo.blocker != "user-1" || repo.target != "user-2" {
		t.Fatalf("expected block success, got %d, %v, %#v", status, err, repo)
	}
}

func TestUnblockUserValidatesAndReturnsDependencyError(t *testing.T) {
	repo := &fakeBlockRepoService{err: errors.New("database unavailable")}
	status, _, err := BlockServiceInit(repo).UnblockUser(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"target":"user-2"}`, "user-1"))
	if status != http.StatusInternalServerError || err == nil {
		t.Fatalf("expected unblock dependency error, got %d, %v", status, err)
	}
}
