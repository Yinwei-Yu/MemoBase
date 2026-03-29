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
	QdrantURL        string
	QdrantCollection string
	EmbeddingDim     int
	OllamaURL        string
	OllamaChatModel  string
	OllamaEmbedModel string
	OllamaTimeout    time.Duration
	BM25Weight       float64
	VectorWeight     float64
	RetrieveLimit    int
	EnableDemoUser   bool
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
		QdrantURL:        get("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection: get("QDRANT_COLLECTION", "kb_chunks"),
		EmbeddingDim:     getInt("EMBEDDING_DIM", 768),
		OllamaURL:        get("OLLAMA_URL", "http://localhost:11434"),
		OllamaChatModel:  get("OLLAMA_CHAT_MODEL", "qwen2.5:3b"),
		OllamaEmbedModel: get("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		OllamaTimeout:    time.Duration(getInt("OLLAMA_TIMEOUT_SEC", 90)) * time.Second,
		BM25Weight:       getFloat("BM25_WEIGHT", 0.5),
		VectorWeight:     getFloat("VECTOR_WEIGHT", 0.5),
		RetrieveLimit:    getInt("RETRIEVAL_DB_CANDIDATE_LIMIT", 5000),
		EnableDemoUser:   getBool("ENABLE_DEMO_USER", enableDemoUserDefault),
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Port) == "" {
		return fmt.Errorf("PORT is required")
	}
	if c.TokenTTL <= 0 {
		return fmt.Errorf("TOKEN_TTL_HOURS must be greater than 0")
	}
	if c.BM25Weight < 0 || c.VectorWeight < 0 {
		return fmt.Errorf("BM25_WEIGHT and VECTOR_WEIGHT must be non-negative")
	}
	if c.BM25Weight == 0 && c.VectorWeight == 0 {
		return fmt.Errorf("BM25_WEIGHT and VECTOR_WEIGHT cannot both be 0")
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
