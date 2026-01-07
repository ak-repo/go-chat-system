package db

import (
	"context"
	"fmt"
	"time"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	DBName         string
	MaxConnections int
}

func NewPostgresDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.Database.Pool.MaxConnections)
	poolConfig.MinConns = int32(cfg.Database.Pool.MinConnections)
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil

}
