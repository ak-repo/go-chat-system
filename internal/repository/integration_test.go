//go:build integration

package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var migrationMu sync.Mutex

func integrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	migrationMu.Lock()
	defer migrationMu.Unlock()
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.users') IS NOT NULL").Scan(&exists); err != nil {
		t.Fatalf("check test schema: %v", err)
	}
	if !exists {
		_, file, _, _ := runtime.Caller(0)
		migrationPath := filepath.Join(filepath.Dir(file), "..", "..", "migrations", "20260829120500_canonical_schema.sql")
		sql, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read canonical migration: %v", err)
		}
		up := strings.SplitN(string(sql), "-- +goose Down", 2)[0]
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatalf("apply canonical migration: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE conversation_read_state, message_deliveries, messages, conversation_members, conversations, account_tokens, sessions, friend_requests, blocks, friends, users CASCADE`); err != nil {
		t.Fatalf("reset test database: %v", err)
	}
	return pool
}

func integrationUsers(t *testing.T, db *pgxpool.Pool, count int) []string {
	t.Helper()
	ids := make([]string, count)
	for i := range ids {
		ids[i] = uuid.NewString()
		_, err := db.Exec(context.Background(), `INSERT INTO users(id,username,email,password_hash,role) VALUES($1,$2,$3,'hash','user')`, ids[i], "user"+ids[i][:8], ids[i]+"@example.com")
		if err != nil {
			t.Fatalf("insert test user: %v", err)
		}
	}
	return ids
}

func testSession(userID string) *model.Session {
	return &model.Session{ID: uuid.NewString(), UserID: userID, RefreshTokenHash: []byte(uuid.NewString()), ExpiresAt: time.Now().UTC().Add(time.Hour)}
}
