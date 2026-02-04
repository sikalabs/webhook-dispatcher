package storage

import (
	"context"
	"time"
)

// Event represents a webhook event stored in the database
type Event struct {
	Key       string    `bson:"key" json:"key"`
	Path      string    `bson:"path" json:"path"`
	Body      string    `bson:"body" json:"body"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
}

// Storage is the interface for storing webhook events
type Storage interface {
	// Store saves a webhook event
	Store(ctx context.Context, key string, path string, body string) error

	// Count returns the number of stored events
	Count(ctx context.Context) (int64, error)

	// Close closes the storage connection
	Close() error
}
