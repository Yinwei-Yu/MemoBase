package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"memobase/backend/internal/config"
	"memobase/backend/internal/core"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	logger := NewLogger("dev")
	if logger == nil {
		t.Fatalf("logger is nil")
	}
}

func TestNewServerHealthz(t *testing.T) {
	t.Parallel()

	app := &core.App{
		Config: config.Config{
			AppEnv:     "test",
			CORSOrigin: "http://localhost:5173",
			JWTSecret:  "test-secret",
		},
		Logger: slog.Default(),
	}

	r := NewServer(app)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("X-Request-Id"); got == "" {
		t.Fatalf("X-Request-Id should be set")
	}
}
