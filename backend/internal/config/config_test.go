package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("TOKEN_TTL_HOURS", "")
	t.Setenv("EMBEDDING_DIM", "")
	t.Setenv("BM25_WEIGHT", "")
	t.Setenv("VECTOR_WEIGHT", "")

	cfg := Load()
	if cfg.AppEnv != "dev" {
		t.Fatalf("AppEnv = %q; want %q", cfg.AppEnv, "dev")
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q; want %q", cfg.Port, "8080")
	}
	if cfg.TokenTTL != 2*time.Hour {
		t.Fatalf("TokenTTL = %v; want %v", cfg.TokenTTL, 2*time.Hour)
	}
	if cfg.EmbeddingDim != 768 {
		t.Fatalf("EmbeddingDim = %d; want %d", cfg.EmbeddingDim, 768)
	}
}

func TestLoadWithEnvOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("PORT", "18080")
	t.Setenv("TOKEN_TTL_HOURS", "6")
	t.Setenv("BM25_WEIGHT", "0.3")
	t.Setenv("VECTOR_WEIGHT", "0.7")

	cfg := Load()
	if cfg.AppEnv != "prod" {
		t.Fatalf("AppEnv = %q; want %q", cfg.AppEnv, "prod")
	}
	if cfg.Port != "18080" {
		t.Fatalf("Port = %q; want %q", cfg.Port, "18080")
	}
	if cfg.TokenTTL != 6*time.Hour {
		t.Fatalf("TokenTTL = %v; want %v", cfg.TokenTTL, 6*time.Hour)
	}
	if cfg.BM25Weight != 0.3 {
		t.Fatalf("BM25Weight = %f; want 0.3", cfg.BM25Weight)
	}
	if cfg.VectorWeight != 0.7 {
		t.Fatalf("VectorWeight = %f; want 0.7", cfg.VectorWeight)
	}
}

func TestInvalidNumericEnvFallsBack(t *testing.T) {
	t.Setenv("TOKEN_TTL_HOURS", "not-int")
	t.Setenv("EMBEDDING_DIM", "bad")
	t.Setenv("BM25_WEIGHT", "bad")

	cfg := Load()
	if cfg.TokenTTL != 2*time.Hour {
		t.Fatalf("TokenTTL = %v; want %v", cfg.TokenTTL, 2*time.Hour)
	}
	if cfg.EmbeddingDim != 768 {
		t.Fatalf("EmbeddingDim = %d; want %d", cfg.EmbeddingDim, 768)
	}
	if cfg.BM25Weight != 0.5 {
		t.Fatalf("BM25Weight = %f; want 0.5", cfg.BM25Weight)
	}
}
