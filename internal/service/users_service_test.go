package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
)

type fakeUserRepo struct {
	user           *model.User
	getErr         error
	profileID      string
	passwordID     string
	passwordHash   string
	deactivatedID  string
	profileErr     error
	passwordErr    error
	deactivateErr  error
	searchErr      error
	searchedFilter string
	searchedLimit  int
	created        *model.User
	createErr      error
	sessionErr     error
	rotateErr      error
	revokeID       string
	revokeErr      error
	revokeUserID   string
	revokeUserErr  error
}

func (f *fakeUserRepo) SearchUser(_ context.Context, filter string, limit int) (model.UsersDTO, error) {
	f.searchedFilter, f.searchedLimit = filter, limit
	return nil, f.searchErr
}
func (f *fakeUserRepo) CreateUser(_ context.Context, user *model.User) error {
	f.created = user
	return f.createErr
}
func (f *fakeUserRepo) GetByEmail(context.Context, string) (*model.User, error) {
	return f.user, f.getErr
}
func (f *fakeUserRepo) GetByID(_ context.Context, id string) (*model.User, error) {
	f.profileID = id
	return f.user, f.getErr
}
func (f *fakeUserRepo) CreateSession(context.Context, *model.Session) error { return f.sessionErr }
func (f *fakeUserRepo) RotateSession(context.Context, []byte, *model.Session, time.Time) error {
	return f.rotateErr
}
func (f *fakeUserRepo) RevokeSession(_ context.Context, id string) error {
	f.revokeID = id
	return f.revokeErr
}
func (f *fakeUserRepo) RevokeUserSessions(_ context.Context, id string) error {
	f.revokeUserID = id
	return f.revokeUserErr
}
func (f *fakeUserRepo) IsSessionActive(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *fakeUserRepo) CreateAccountToken(context.Context, string, string, []byte, time.Time) (string, error) {
	return "token-id", nil
}
func (f *fakeUserRepo) ConsumeAccountToken(context.Context, string, []byte) (string, error) {
	return f.user.ID, nil
}
func (f *fakeUserRepo) VerifyAccount(context.Context, []byte) (string, error) { return f.user.ID, nil }
func (f *fakeUserRepo) UpdateProfile(_ context.Context, id, _, _ string) error {
	f.profileID = id
	return f.profileErr
}
func (f *fakeUserRepo) ChangePassword(_ context.Context, id, hash string) error {
	f.passwordID, f.passwordHash = id, hash
	return f.passwordErr
}
func (f *fakeUserRepo) Deactivate(_ context.Context, id string) error {
	f.deactivatedID = id
	return f.deactivateErr
}

func authenticatedRequest(method, body, id string) *http.Request {
	r := httptest.NewRequest(method, "/", bytes.NewBufferString(body))
	return r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, id))
}

func TestUserGetMeUsesAuthenticatedIdentity(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1", Username: "alice", Email: "alice@example.com", Role: "user"}}
	status, response, err := NewUserServiceImpl(repo).GetMe(httptest.NewRecorder(), authenticatedRequest(http.MethodGet, "", "user-1"))
	if err != nil || status != http.StatusOK || response == nil {
		t.Fatalf("expected successful profile response, got %d, %v", status, err)
	}
	if repo.profileID != "user-1" {
		t.Fatalf("expected authenticated ID, got %q", repo.profileID)
	}
}

func TestUserUpdateMeRequiresAuthenticationAndPropagatesDependencyError(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1"}, profileErr: errors.New("database unavailable")}
	service := NewUserServiceImpl(repo)
	status, _, err := service.UpdateMe(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString(`{"username":"alice","email":"alice@example.com"}`)))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %d, %v", status, err)
	}
	status, _, err = service.UpdateMe(httptest.NewRecorder(), authenticatedRequest(http.MethodPut, `{"username":"alice","email":"alice@example.com"}`, "user-1"))
	if status != http.StatusInternalServerError || err == nil || repo.profileID != "user-1" {
		t.Fatalf("expected authenticated dependency failure, got %d, %v", status, err)
	}
}

func TestUserChangePasswordValidatesAndUsesAuthenticatedIdentity(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1"}}
	service := NewUserServiceImpl(repo)
	status, _, err := service.ChangePassword(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"password":"short"}`, "user-1"))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrWeakPassword) {
		t.Fatalf("expected weak password validation, got %d, %v", status, err)
	}
	status, _, err = service.ChangePassword(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"password":"strong-password"}`, "user-1"))
	if status != http.StatusOK || err != nil || repo.passwordID != "user-1" || repo.passwordHash == "" {
		t.Fatalf("expected password change, got %d, %v", status, err)
	}
}

func TestUserSearchValidatesDefaultsAndPropagatesErrors(t *testing.T) {
	repo := &fakeUserRepo{}
	service := NewUserServiceImpl(repo)
	status, _, err := service.SearchUser(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?filter=alice&limit=7", nil))
	if status != http.StatusOK || err != nil || repo.searchedFilter != "alice" || repo.searchedLimit != 7 {
		t.Fatalf("unexpected search: %d %v %#v", status, err, repo)
	}
	status, _, err = service.SearchUser(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?filter="+strings.Repeat("a", 129), nil))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected filter validation, got %d %v", status, err)
	}
	repo.searchErr = errors.New("search failed")
	status, _, err = service.SearchUser(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?filter=ab&limit=bad", nil))
	if status != http.StatusInternalServerError || err == nil || repo.searchedLimit != 20 {
		t.Fatalf("expected search dependency error, got %d %v", status, err)
	}
}

func TestUserRegisterLoginRefreshLogoutAndDeactivate(t *testing.T) {
	repo := &fakeUserRepo{user: &model.User{ID: "user-1", Email: "alice@example.com", Username: "alice", Role: "user"}}
	service := NewUserServiceImpl(repo)
	status, response, err := service.Register(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"username":" alice ","email":"ALICE@EXAMPLE.COM","password":"strong-password"}`)))
	if status != http.StatusCreated || response == nil || err != nil || repo.created == nil || repo.created.Email != "alice@example.com" {
		t.Fatalf("expected registration, got %d %v", status, err)
	}
	status, _, err = service.Register(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"username":"alice","email":"bad","password":"strong-password"}`)))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrInvalidEmail) {
		t.Fatalf("expected invalid email, got %d %v", status, err)
	}
	repo.user.PasswordHash, _ = utils.HashPassword("strong-password")
	status, _, err = service.Login(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":" ALICE@EXAMPLE.COM ","password":"strong-password"}`)))
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected login, got %d %v", status, err)
	}
	status, _, err = service.Login(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":"alice@example.com","password":"wrong-password"}`)))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected failed login, got %d %v", status, err)
	}
	refresh, _, session, _ := newRefresh("user-1")
	status, _, err = service.RefreshToken(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refresh_token":"bad"}`)))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected invalid refresh, got %d %v", status, err)
	}
	_ = session
	status, _, err = service.RefreshToken(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refresh_token":"`+refresh+`"}`)))
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected refresh, got %d %v", status, err)
	}
	logout := authenticatedRequest(http.MethodPost, `{"refresh_token":"`+refresh+`"}`, "user-1")
	status, _, err = service.Logout(httptest.NewRecorder(), logout)
	if status != http.StatusOK || err != nil || repo.revokeID == "" {
		t.Fatalf("expected logout, got %d %v", status, err)
	}
	status, _, err = service.Deactivate(httptest.NewRecorder(), authenticatedRequest(http.MethodDelete, "", "user-1"))
	if status != http.StatusOK || err != nil || repo.deactivatedID != "user-1" {
		t.Fatalf("expected deactivation, got %d %v", status, err)
	}
}

func TestUserLogoutAndDeactivateRequireAuthentication(t *testing.T) {
	service := NewUserServiceImpl(&fakeUserRepo{})
	status, _, err := service.Logout(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`)))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected logout auth error, got %d %v", status, err)
	}
	status, _, err = service.Deactivate(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/", nil))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected deactivate auth error, got %d %v", status, err)
	}
}
