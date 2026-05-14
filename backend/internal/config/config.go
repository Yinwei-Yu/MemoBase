package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv           string
	Port             string
	CORSOrigin       string
	JWTSecret        string
	TokenTTL         time.Duration
	DatabaseURL      string
	StorageDir       string
	QdrantCollection string
	EnableDemoUser   bool
	AgentServiceURL  string
	ContextWindow    int // model context window size, default 4096
	OutputReserve    int // reserved tokens for output, default 1024

	// Memory management
	MemoryMaxShortTerm        int
	MemoryMaxLongTerm         int
	MemoryConsolidateInterval time.Duration
}

func Load() Config {
	appEnv := get("APP_ENV", "dev")
	enableDemoUserDefault := appEnv == "dev"

	return Config{
		AppEnv:           appEnv,
		Port:             get("PORT", "8080"),
		CORSOrigin:       get("CORS_ORIGIN", "http://localhost:5173"),
		JWTSecret:        get("JWT_SECRET", "memo-dev-secret"),
		TokenTTL:         time.Duration(getInt("TOKEN_TTL_HOURS", 2)) * time.Hour,
		DatabaseURL:      get("DATABASE_URL", "postgres://memo:memo@localhost:5432/memo?sslmode=disable"),
		StorageDir:       get("STORAGE_DIR", "./storage"),
		QdrantCollection: get("QDRANT_COLLECTION", "kb_chunks"),
		EnableDemoUser:   getBool("ENABLE_DEMO_USER", enableDemoUserDefault),
		AgentServiceURL:  get("AGENT_SERVICE_URL", "localhost:50051"),
		ContextWindow:    getInt("CONTEXT_WINDOW", 4096),
		OutputReserve:    getInt("OUTPUT_RESERVE", 1024),

		MemoryMaxShortTerm:        getInt("MEMORY_MAX_SHORT_TERM", 50),
		MemoryMaxLongTerm:         getInt("MEMORY_MAX_LONG_TERM", 200),
		MemoryConsolidateInterval: time.Duration(getInt("MEMORY_CONSOLIDATE_INTERVAL_MINUTES", 60)) * time.Minute,
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("PORT is required")
	}
	if c.TokenTTL <= 0 {
		return fmt.Errorf("TOKEN_TTL_HOURS must be greater than 0")
	}
	if c.AppEnv != "dev" {
		if strings.TrimSpace(c.JWTSecret) == "" {
			return fmt.Errorf("JWT_SECRET is required in non-dev environments")
		}
		if c.JWTSecret == "memo-dev-secret" {
			return fmt.Errorf("JWT_SECRET cannot use default value in non-dev environments")
		}
	}
	return nil
}

func get(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
