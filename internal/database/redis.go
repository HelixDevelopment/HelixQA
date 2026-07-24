package database

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	Client *redis.Client
}

func NewRedis(redisURL string) (*RedisDB, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("unable to ping redis: %w", err)
	}

	return &RedisDB{Client: client}, nil
}

func (r *RedisDB) HealthCheck(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}

func (r *RedisDB) Close() {
	r.Client.Close()
}
