package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("TOKEN_TTL_HOURS", "")

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
}

func TestLoadWithEnvOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("PORT", "18080")
	t.Setenv("TOKEN_TTL_HOURS", "6")

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
}

func TestInvalidNumericEnvFallsBack(t *testing.T) {
	t.Setenv("TOKEN_TTL_HOURS", "not-int")

	cfg := Load()
	if cfg.TokenTTL != 2*time.Hour {
		t.Fatalf("TokenTTL = %v; want %v", cfg.TokenTTL, 2*time.Hour)
	}
}
