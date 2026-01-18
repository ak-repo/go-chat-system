package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBconnection struct {
	Pool *pgxpool.Pool
}

var DB DBconnection

func ConnectDB() error {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.Config.Database.User,
		config.Config.Database.Password,
		config.Config.Database.Host,
		config.Config.Database.Port,
		config.Config.Database.Name,
		config.Config.Database.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse pool config: %w", err)
	}

	poolConfig.MaxConns = int32(config.Config.Database.Pool.MaxConnections)
	poolConfig.MinConns = int32(config.Config.Database.Pool.MinConnections)
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	ctx := context.Background()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	DB.Pool = pool

	return nil

}

func GetDB() *pgxpool.Pool {
	return DB.Pool
}
