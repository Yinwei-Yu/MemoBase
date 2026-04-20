package api

import (
	"encoding/json"
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
	"github.com/golang-jwt/jwt/v5"
)

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("uses incoming request id", func(t *testing.T) {
		t.Parallel()
		r := gin.New()
		r.Use(RequestID())
		r.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"request_id": c.GetString("request_id")})
		})

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Request-Id", "req_custom")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
		if got := w.Header().Get("X-Request-Id"); got != "req_custom" {
			t.Fatalf("X-Request-Id header = %q; want %q", got, "req_custom")
		}
	})

	t.Run("generates id when header is missing", func(t *testing.T) {
		t.Parallel()
		r := gin.New()
		r.Use(RequestID())
		r.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"request_id": c.GetString("request_id")})
		})

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
		got := w.Header().Get("X-Request-Id")
		if !strings.HasPrefix(got, "req_") {
			t.Fatalf("generated request id = %q; want prefix req_", got)
		}
	})
}

func TestCorsMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Cors("http://localhost:5173"))
	nextCalled := false
	r.GET("/ok", func(c *gin.Context) {
		nextCalled = true
		c.Status(http.StatusOK)
	})

	wGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GET status = %d; want %d", wGet.Code, http.StatusOK)
	}
	if !nextCalled {
		t.Fatalf("expected next handler to be called for GET")
	}
	if got := wGet.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow origin = %q; want %q", got, "http://localhost:5173")
	}

	wOptions := httptest.NewRecorder()
	reqOptions := httptest.NewRequest(http.MethodOptions, "/ok", nil)
	r.ServeHTTP(wOptions, reqOptions)
	if wOptions.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d; want %d", wOptions.Code, http.StatusNoContent)
	}
}

func TestAuthRequiredMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	app := &core.App{
		Config: config.Config{
			JWTSecret: "test-secret",
		},
		Logger: slog.Default(),
	}
	r := gin.New()
	r.GET("/me", AuthRequired(app), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})

	t.Run("missing bearer token", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid subject claim", func(t *testing.T) {
		t.Parallel()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": 123})
		signed, err := token.SignedString([]byte("test-secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()
		token, err := util.SignToken("test-secret", "u_123", time.Hour)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if body["user_id"] != "u_123" {
			t.Fatalf("user_id = %q; want %q", body["user_id"], "u_123")
		}
	})
}
