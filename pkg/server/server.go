package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

// Server starts the webhook server
func Server() {
	// Get Redis address from environment or use default
	redisAddr := os.Getenv("REDIS")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test Redis connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", redisAddr, err)
	}
	log.Printf("Connected to Redis at %s", redisAddr)

	// Create HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(w, r, rdb)
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting webhook server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleWebhook processes incoming webhook requests
func handleWebhook(w http.ResponseWriter, r *http.Request, rdb *redis.Client) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		log.Printf("Error reading body: %v", err)
		return
	}
	defer r.Body.Close()

	// Parse body as JSON (validate it's valid JSON)
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.Printf("Invalid JSON from %s: %v", r.RemoteAddr, err)
		return
	}

	// Slugify the path
	slugifiedPath := slugify(r.URL.Path)

	// Generate Redis key: webhook-<slugified-path>-<unix-timestamp>
	unixTime := time.Now().Unix()
	key := fmt.Sprintf("webhook-%s-%d", slugifiedPath, unixTime)

	// Store in Redis
	err = rdb.Set(ctx, key, body, 0).Err()
	if err != nil {
		http.Error(w, "Failed to store webhook", http.StatusInternalServerError)
		log.Printf("Failed to store in Redis: %v", err)
		return
	}

	log.Printf("Stored webhook: %s (path: %s, size: %d bytes)", key, r.URL.Path, len(body))

	// Send success response
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Webhook received and stored: %s\n", key)
}

// slugify converts a path into a slug suitable for Redis keys
func slugify(path string) string {
	// Remove leading/trailing slashes
	path = strings.Trim(path, "/")

	// If empty path, use "root"
	if path == "" {
		return "root"
	}

	// Replace slashes with hyphens
	path = strings.ReplaceAll(path, "/", "-")

	// Remove or replace special characters
	reg := regexp.MustCompile("[^a-zA-Z0-9-_]+")
	path = reg.ReplaceAllString(path, "-")

	// Remove consecutive hyphens
	reg = regexp.MustCompile("-+")
	path = reg.ReplaceAllString(path, "-")

	// Trim hyphens from start/end
	path = strings.Trim(path, "-")

	// Convert to lowercase
	path = strings.ToLower(path)

	return path
}
