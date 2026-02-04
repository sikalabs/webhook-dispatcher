package storage

import (
	"context"
	"log"
)

// DualStorage implements Storage interface for both Redis and MongoDB
type DualStorage struct {
	redis   *RedisStorage
	mongodb *MongoDBStorage
}

// NewDualStorage creates a new dual storage backend (Redis + MongoDB)
func NewDualStorage(redis *RedisStorage, mongodb *MongoDBStorage) *DualStorage {
	return &DualStorage{
		redis:   redis,
		mongodb: mongodb,
	}
}

// Store saves a webhook event to both Redis and MongoDB
func (d *DualStorage) Store(ctx context.Context, key string, path string, body string) error {
	// Store in Redis first (primary storage)
	if err := d.redis.Store(ctx, key, path, body); err != nil {
		return err
	}

	// Store in MongoDB (secondary storage)
	// Log error but don't fail the request if MongoDB fails
	if err := d.mongodb.Store(ctx, key, path, body); err != nil {
		log.Printf("Warning: Failed to store in MongoDB: %v", err)
	}

	return nil
}

// Count returns the count from Redis (primary storage)
func (d *DualStorage) Count(ctx context.Context) (int64, error) {
	return d.redis.Count(ctx)
}

// RedisCount returns the count from Redis
func (d *DualStorage) RedisCount(ctx context.Context) (int64, error) {
	return d.redis.Count(ctx)
}

// MongoDBCount returns the count from MongoDB
func (d *DualStorage) MongoDBCount(ctx context.Context) (int64, error) {
	return d.mongodb.Count(ctx)
}

// Close closes both storage connections
func (d *DualStorage) Close() error {
	// Close both connections, log errors but continue
	var redisErr, mongoErr error

	if err := d.redis.Close(); err != nil {
		log.Printf("Error closing Redis connection: %v", err)
		redisErr = err
	}

	if err := d.mongodb.Close(); err != nil {
		log.Printf("Error closing MongoDB connection: %v", err)
		mongoErr = err
	}

	// Return the first error encountered
	if redisErr != nil {
		return redisErr
	}
	return mongoErr
}
