package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBStorage implements Storage interface for MongoDB
type MongoDBStorage struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewMongoDBStorage creates a new MongoDB storage backend
func NewMongoDBStorage(uri string, database string, collection string) (*MongoDBStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	coll := client.Database(database).Collection(collection)

	return &MongoDBStorage{
		client:     client,
		collection: coll,
	}, nil
}

// Store saves a webhook event to MongoDB
func (m *MongoDBStorage) Store(ctx context.Context, key string, path string, body string) error {
	event := Event{
		Key:       key,
		Path:      path,
		Body:      body,
		Timestamp: time.Now(),
	}

	_, err := m.collection.InsertOne(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to insert event to MongoDB: %w", err)
	}

	return nil
}

// Close closes the MongoDB connection
func (m *MongoDBStorage) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}
