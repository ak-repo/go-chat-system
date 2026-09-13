//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/google/uuid"
)

func TestUserRepositoryCRUDAndSearch(t *testing.T) {
	db := integrationDB(t)
	integrationUsers(t, db, 2)
	r := NewUserRepositoryImpl(db)

	u := &model.User{ID: uuid.NewString(), Username: "Alice", Email: "Alice@Example.com", PasswordHash: "hash", Role: "user", CreatedAt: time.Now().UTC(), ModifiedAt: time.Now().UTC()}
	if err := r.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetByID(context.Background(), u.ID)
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("get user: got %#v, err %v", got, err)
	}
	got, err = r.GetByEmail(context.Background(), "alice@example.com")
	if err != nil || got == nil || got.ID != u.ID {
		t.Fatalf("get user by email: got %#v, err %v", got, err)
	}
	if _, err := r.GetByEmail(context.Background(), "missing@example.com"); err != nil {
		t.Fatalf("missing email should return nil error: %v", err)
	}
	results, err := r.SearchUser(context.Background(), "example", 10)
	if err != nil || len(results) != 3 {
		t.Fatalf("search users: got %d, err %v", len(results), err)
	}
	if empty, err := r.SearchUser(context.Background(), "u", 10); err != nil || len(empty) != 0 {
		t.Fatalf("short search should be empty: %v, %v", empty, err)
	}
}

func TestUserRepositorySessionsAndAccountTokens(t *testing.T) {
	db := integrationDB(t)
	uid := integrationUsers(t, db, 1)[0]
	r := NewUserRepositoryImpl(db)
	s := testSession(uid)
	if err := r.CreateSession(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	active, err := r.IsSessionActive(context.Background(), uid, s.ID)
	if err != nil || !active {
		t.Fatalf("session should be active: %v, %v", active, err)
	}
	if err := r.RotateSession(context.Background(), s.RefreshTokenHash, testSession(uid), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := r.RotateSession(context.Background(), s.RefreshTokenHash, testSession(uid), time.Now().UTC()); err == nil || !errs.Is(err, errs.ErrTokenReused) {
		t.Fatalf("expected token reuse error, got %v", err)
	}

	hash := []byte(uuid.NewString())
	if _, err := r.CreateAccountToken(context.Background(), uid, "verification", hash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, err := r.VerifyAccount(context.Background(), hash); err != nil || got != uid {
		t.Fatalf("verify account: %q, %v", got, err)
	}
	if _, err := r.VerifyAccount(context.Background(), hash); err == nil {
		t.Fatal("verification token should be single-use")
	}
}
