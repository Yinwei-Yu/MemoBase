package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memobase/backend/internal/config"
	"memobase/backend/internal/core"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func newRouteTestEngine() (*gin.Engine, *core.App) {
	gin.SetMode(gin.TestMode)
	app := &core.App{
		Config: config.Config{
			JWTSecret:       "test-secret",
			TokenTTL:        2 * time.Hour,
			OllamaChatModel: "qwen2.5:3b",
		},
		Logger: slog.Default(),
	}
	r := gin.New()
	RegisterRoutes(r, app)
	return r, app
}

func authHeader(t *testing.T, app *core.App, userID string) string {
	t.Helper()
	token, err := util.SignToken(app.Config.JWTSecret, userID, app.Config.TokenTTL)
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	return "Bearer " + token
}

func TestLoginRouteValidation(t *testing.T) {
	t.Parallel()
	r, _ := newRouteTestEngine()

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":1}`))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid payload status = %d; want %d", w1.Code, http.StatusUnprocessableEntity)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"", "password":" "}`))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty credentials status = %d; want %d", w2.Code, http.StatusUnprocessableEntity)
	}
}

func TestPublicRoutes(t *testing.T) {
	t.Parallel()
	r, _ := newRouteTestEngine()

	wHealth := httptest.NewRecorder()
	reqHealth := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	r.ServeHTTP(wHealth, reqHealth)
	if wHealth.Code != http.StatusOK {
		t.Fatalf("healthz status = %d; want %d", wHealth.Code, http.StatusOK)
	}

	wMetrics := httptest.NewRecorder()
	reqMetrics := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	r.ServeHTTP(wMetrics, reqMetrics)
	if wMetrics.Code != http.StatusOK {
		t.Fatalf("metrics status = %d; want %d", wMetrics.Code, http.StatusOK)
	}
	if !strings.Contains(wMetrics.Body.String(), "memobase_up 1") {
		t.Fatalf("metrics body missing expected sample: %q", wMetrics.Body.String())
	}
}

func TestAuthedRoutesRejectMissingToken(t *testing.T) {
	t.Parallel()
	r, _ := newRouteTestEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestCreateSessionValidation(t *testing.T) {
	t.Parallel()
	r, app := newRouteTestEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"kb_id":"", "title":" "}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader(t, app, "u_demo"))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
	}
}

func TestChatCompletionValidation(t *testing.T) {
	t.Parallel()
	r, app := newRouteTestEngine()
	auth := authHeader(t, app, "u_demo")

	t.Run("empty kb/question", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewBufferString(`{"kb_id":"", "question":" "}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", auth)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("question too long", func(t *testing.T) {
		t.Parallel()
		longQuestion := strings.Repeat("a", 2001)
		payload := `{"kb_id":"kb_1","question":"` + longQuestion + `"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", auth)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})
}
