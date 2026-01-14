package database

import (
	"context"
	"fmt"
	"log"

	"github.com/ak-repo/go-chat-system/config"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func InitRedis() {

	cfg := config.Config.Redis

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       0,
	})

	ctx := context.Background()
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Could not connect to Redis: ", err)
	}
	log.Println("Redis connected successfully")

}
