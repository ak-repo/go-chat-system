package clients

import (
	"fmt"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg *config.Redis) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
