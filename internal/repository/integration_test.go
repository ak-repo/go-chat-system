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

type integrationMigration struct {
	filename string
	applied  func(context.Context, *pgxpool.Pool) (bool, error)
}

func applyIntegrationMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate integration test source")
	}
	migrationsDir := filepath.Join(filepath.Dir(file), "..", "..", "migrations")
	migrations := []integrationMigration{
		{"20260829120500_canonical_schema.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, "SELECT to_regclass('public.users') IS NOT NULL").Scan(&exists)
			return exists, err
		}},
		{"20260925090000_conversation_user_state.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, "SELECT to_regclass('public.conversation_user_state') IS NOT NULL").Scan(&exists)
			return exists, err
		}},
		{"20260927120000_group_conversations.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='conversations' AND column_name='creator_id')`).Scan(&exists)
			return exists, err
		}},
		{"20260927150000_message_interactions.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, "SELECT to_regclass('public.message_reactions') IS NOT NULL").Scan(&exists)
			return exists, err
		}},
		{"20260927180000_mentions_notifications.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, "SELECT to_regclass('public.notifications') IS NOT NULL").Scan(&exists)
			return exists, err
		}},
		{"20260927210000_group_invites.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, "SELECT to_regclass('public.group_invites') IS NOT NULL").Scan(&exists)
			return exists, err
		}},
		{"20260927230000_phase2_integrity_fixes.sql", func(ctx context.Context, db *pgxpool.Pool) (bool, error) {
			var exists bool
			err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='notifications_message_conversation_fkey')`).Scan(&exists)
			return exists, err
		}},
	}
	for _, migration := range migrations {
		applied, err := migration.applied(ctx, pool)
		if err != nil {
			t.Fatalf("check migration %s: %v", migration.filename, err)
		}
		if applied {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(migrationsDir, migration.filename))
		if err != nil {
			t.Fatalf("read migration %s: %v", migration.filename, err)
		}
		parts := strings.SplitN(string(contents), "-- +goose Down", 2)
		if len(parts) != 2 {
			t.Fatalf("migration %s has no goose Down section", migration.filename)
		}
		if _, err = pool.Exec(ctx, parts[0]); err != nil {
			t.Fatalf("apply migration %s: %v", migration.filename, err)
		}
	}
}

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
	applyIntegrationMigrations(t, ctx, pool)
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE group_invites, notifications, notification_preferences, message_mentions, message_reactions, conversation_read_state, conversation_user_state, message_deliveries, messages, conversation_members, conversations, account_tokens, sessions, friend_requests, blocks, friends, users CASCADE`); err != nil {
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
