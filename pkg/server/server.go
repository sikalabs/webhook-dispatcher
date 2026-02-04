package server

import (
	"bytes"
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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sikalabs/webhook-dispatcher/pkg/storage"
	"gopkg.in/yaml.v3"
)

var ctx = context.Background()
var enableLogging bool

var (
	redisEventsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_dispatcher_redis_events_total",
		Help: "Total number of events stored in Redis",
	})
	mongodbEventsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "webhook_dispatcher_mongodb_events_total",
		Help: "Total number of events stored in MongoDB",
	})
)

func init() {
	prometheus.MustRegister(redisEventsGauge)
	prometheus.MustRegister(mongodbEventsGauge)
}

// Config represents the webhook dispatch configuration
type Config struct {
	Meta struct {
		SchemaVersion int `yaml:"SchemaVersion"`
	} `yaml:"Meta"`
	Dispatch []DispatchRule `yaml:"Dispatch"`
}

// DispatchRule represents a single dispatch rule
type DispatchRule struct {
	Path    string   `yaml:"Path"`
	Targets []string `yaml:"Targets"`
}

// Server starts the webhook server
func Server() {
	// Check if logging is enabled
	enableLogging = os.Getenv("LOG") == "1"
	if enableLogging {
		log.Printf("Request logging enabled")
	}

	// Load config
	configPath := os.Getenv("CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	config, err := loadConfig(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config from %s: %v", configPath, err)
		log.Printf("Continuing without dispatch rules")
		config = &Config{}
	} else {
		log.Printf("Loaded config from %s with %d dispatch rules", configPath, len(config.Dispatch))
	}

	// Initialize Redis storage (always required)
	redisHost := os.Getenv("REDIS")
	if redisHost == "" {
		redisHost = "127.0.0.1"
	}

	redisStore, err := storage.NewRedisStorage(redisHost)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Connected to Redis at %s:6379", redisHost)

	// Check if MongoDB is also configured
	mongoURI := os.Getenv("MONGODB_URI")
	var store storage.Storage
	var mongoStore *storage.MongoDBStorage

	if mongoURI != "" {
		// MongoDB is configured, use dual storage (Redis + MongoDB)
		mongoDatabase := os.Getenv("MONGODB_DATABASE")
		if mongoDatabase == "" {
			mongoDatabase = "webhook-dispatcher"
		}
		mongoCollection := os.Getenv("MONGODB_COLLECTION")
		if mongoCollection == "" {
			mongoCollection = "events"
		}

		var err error
		mongoStore, err = storage.NewMongoDBStorage(mongoURI, mongoDatabase, mongoCollection)
		if err != nil {
			log.Fatalf("Failed to connect to MongoDB: %v", err)
		}
		log.Printf("Connected to MongoDB at %s (database: %s, collection: %s)", mongoURI, mongoDatabase, mongoCollection)

		// Use dual storage
		store = storage.NewDualStorage(redisStore, mongoStore)
		log.Printf("Using dual storage: Redis (primary) + MongoDB (secondary)")
	} else {
		// Use only Redis
		store = redisStore
		log.Printf("Using Redis storage only")
	}

	defer store.Close()

	// Start metrics collection goroutine
	go updateMetrics(redisStore, mongoStore)

	// Create HTTP handlers
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Show homepage for GET requests to root path
		if r.Method == "GET" && r.URL.Path == "/" {
			handleHomepage(w, r)
			return
		}
		handleWebhook(w, r, store, config)
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

// loadConfig loads and parses the config file
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// handleHomepage serves the homepage
func handleHomepage(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Webhook Dispatcher</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
            color: #333;
        }
        h1 {
            color: #2c3e50;
        }
        .description {
            font-size: 1.1em;
            margin: 20px 0;
        }
        .status {
            background-color: #d4edda;
            border: 1px solid #c3e6cb;
            color: #155724;
            padding: 12px;
            border-radius: 4px;
            margin-top: 20px;
        }
        .right {
            position: fixed;
            bottom: 0px;
            right: 20px;
            font-size: 1.2em;
        }
    </style>
</head>
<body>
    <h1>Webhook Dispatcher</h1>
    <div class="description">
        <p>A simple webhook receiver that stores webhook payloads and can forward them to configured targets.</p>
    </div>
    <div class="status">
        <strong>Status:</strong> Service is running and ready to receive webhooks
    </div>
    <p class="right">
        <a href="https://github.com/sikalabs/webhook-dispatcher" target="_blank" style="color:black">webhook-dispatcher</a> by <a href="https://sikalabs.com" target="_blank" style="color:black">sikalabs</a>
    </p>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, html)
}

// handleWebhook processes incoming webhook requests
func handleWebhook(w http.ResponseWriter, r *http.Request, store storage.Storage, config *Config) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		log.Printf("Error reading body: %v", err)
		return
	}
	defer r.Body.Close()

	// Log incoming request if enabled
	if enableLogging {
		log.Printf("=== Incoming Request ===")
		log.Printf("Method: %s", r.Method)
		log.Printf("Path: %s", r.URL.Path)
		log.Printf("Remote: %s", r.RemoteAddr)
		log.Printf("Headers:")
		for name, values := range r.Header {
			for _, value := range values {
				log.Printf("  %s: %s", name, value)
			}
		}
		log.Printf("Body: %s", string(body))
		log.Printf("========================")
	}

	// Parse body as JSON (validate it's valid JSON)
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.Printf("Invalid JSON from %s: %v", r.RemoteAddr, err)
		return
	}

	// Slugify the path
	slugifiedPath := slugify(r.URL.Path)

	// Generate key: webhook-<slugified-path>-<unix-timestamp>
	unixTime := time.Now().Unix()
	key := fmt.Sprintf("webhook-%s-%d", slugifiedPath, unixTime)

	// Store in storage backend
	err = store.Store(ctx, key, r.URL.Path, string(body))
	if err != nil {
		http.Error(w, "Failed to store webhook", http.StatusInternalServerError)
		log.Printf("Failed to store webhook: %v", err)
		return
	}

	log.Printf("Stored webhook: %s (path: %s, size: %d bytes)", key, r.URL.Path, len(body))

	// Forward to targets based on dispatch rules
	targets := findTargets(r.URL.Path, config)
	if len(targets) > 0 {
		forwardToTargets(targets, body, r.Header)
	}

	// Send success response
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Webhook received and stored: %s\n", key)
}

// findTargets finds matching targets for the given path
func findTargets(path string, config *Config) []string {
	for _, rule := range config.Dispatch {
		if rule.Path == path {
			return rule.Targets
		}
	}
	return nil
}

// forwardToTargets forwards the webhook to all target URLs
func forwardToTargets(targets []string, body []byte, headers http.Header) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, target := range targets {
		go func(url string) {
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
			if err != nil {
				log.Printf("Failed to create request for %s: %v", url, err)
				return
			}

			// Copy relevant headers
			req.Header.Set("Content-Type", headers.Get("Content-Type"))
			if req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Failed to forward webhook to %s: %v", url, err)
				return
			}
			defer resp.Body.Close()

			log.Printf("Forwarded webhook to %s (status: %d)", url, resp.StatusCode)
		}(target)
	}
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

// updateMetrics periodically updates Prometheus metrics
func updateMetrics(redisStore *storage.RedisStorage, mongoStore *storage.MongoDBStorage) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Update immediately on start
	updateMetricsOnce(redisStore, mongoStore)

	for range ticker.C {
		updateMetricsOnce(redisStore, mongoStore)
	}
}

func updateMetricsOnce(redisStore *storage.RedisStorage, mongoStore *storage.MongoDBStorage) {
	if redisStore != nil {
		count, err := redisStore.Count(ctx)
		if err != nil {
			log.Printf("Failed to get Redis event count: %v", err)
		} else {
			redisEventsGauge.Set(float64(count))
		}
	} else {
		redisEventsGauge.Set(-1)
	}

	if mongoStore != nil {
		count, err := mongoStore.Count(ctx)
		if err != nil {
			log.Printf("Failed to get MongoDB event count: %v", err)
		} else {
			mongodbEventsGauge.Set(float64(count))
		}
	} else {
		mongodbEventsGauge.Set(-1)
	}
}
