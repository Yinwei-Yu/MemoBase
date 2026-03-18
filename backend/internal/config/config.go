package config

import (
	"os"
	"strconv"
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
}

func Load() Config {
	return Config{
		AppEnv:           get("APP_ENV", "dev"),
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
	}
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
