package storage

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements Storage interface for Redis
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage creates a new Redis storage backend
func NewRedisStorage(host string) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:6379", host),
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

// Store saves a webhook event to Redis
func (r *RedisStorage) Store(ctx context.Context, key string, path string, body string) error {
	return r.client.Set(ctx, key, body, 0).Err()
}

// Count returns the number of webhook events stored in Redis
func (r *RedisStorage) Count(ctx context.Context) (int64, error) {
	keys, err := r.client.Keys(ctx, "webhook-*").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count keys: %w", err)
	}
	return int64(len(keys)), nil
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}
