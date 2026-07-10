package database

import (
	"context"
	"fmt"
	"log"

	"github.com/ak-repo/go-chat-system/internal/platform/config"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func InitRedis() error {

	cfg := config.Config.Redis

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       0,
	})

	ctx := context.Background()
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Println("Redis connected successfully")
	return nil

}
