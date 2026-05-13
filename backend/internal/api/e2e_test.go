package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"mime/multipart"
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

// newE2EServer creates a test server with routes registered.
// Uses mock app (no real DB/Qdrant) for API contract testing.
func newE2EServer() (*gin.Engine, *core.App) {
	gin.SetMode(gin.TestMode)
	app := &core.App{
		Config: config.Config{
			JWTSecret:       "e2e-test-secret",
			TokenTTL:        2 * time.Hour,
			OllamaChatModel: "qwen2.5:3b",
		},
		Logger: slog.Default(),
	}
	r := gin.New()
	RegisterRoutes(r, app)
	return r, app
}

func e2eAuthHeader(t *testing.T, app *core.App, userID string) string {
	t.Helper()
	token, err := util.SignToken(app.Config.JWTSecret, userID, app.Config.TokenTTL)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return "Bearer " + token
}

func doJSON(r *gin.Engine, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ---------- Auth E2E ----------

func TestE2E_LoginFlow(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()

	// Login with invalid credentials returns 401
	t.Run("invalid credentials", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/auth/login", gin.H{
			"username": "nobody",
			"password": "wrong",
		}, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	// Login with empty body returns 422
	t.Run("empty body", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/auth/login", gin.H{}, nil)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	// Login with non-JSON body returns 422
	t.Run("non-json body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	// Auth/me without token returns 401
	t.Run("me without token", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	// Auth/me with valid token returns 200
	t.Run("me with valid token", func(t *testing.T) {
		_ = app
		token, _ := util.SignToken(app.Config.JWTSecret, "u_test", app.Config.TokenTTL)
		w := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, map[string]string{
			"Authorization": "Bearer " + token,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
	})

	// Auth/me with malformed token returns 401
	t.Run("me with bad token", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, map[string]string{
			"Authorization": "Bearer garbage.token.value",
		})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	// Auth/me with wrong scheme returns 401
	t.Run("me with wrong scheme", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, map[string]string{
			"Authorization": "Basic dXNlcjpwYXNz",
		})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})
}

// ---------- Health E2E ----------

func TestE2E_HealthEndpoints(t *testing.T) {
	t.Parallel()
	r, _ := newE2EServer()

	t.Run("healthz returns 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/healthz", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data, ok := resp["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("response missing data field")
		}
		if data["status"] != "ok" {
			t.Fatalf("status = %v; want ok", data["status"])
		}
	})

	t.Run("metrics returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
		if !strings.Contains(w.Body.String(), "memobase_up") {
			t.Fatalf("metrics missing memobase_up")
		}
	})
}

// ---------- Knowledge Base E2E ----------

func TestE2E_KnowledgeBaseCRUD(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")
	headers := map[string]string{"Authorization": auth}

	t.Run("create kb validation", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases", gin.H{
			"name":        "Test KB",
			"description": "A test knowledge base",
			"tags":        []string{"test", "e2e"},
		}, headers)
		if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 201 or 500", w.Code)
		}
	})

	t.Run("name too long", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases", gin.H{
			"name": strings.Repeat("a", 65),
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("description too long", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases", gin.H{
			"name":        "ok",
			"description": strings.Repeat("d", 513),
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("too many tags", func(t *testing.T) {
		tags := make([]string, 11)
		for i := range tags {
			tags[i] = "tag"
		}
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases", gin.H{
			"name": "ok",
			"tags": tags,
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases", gin.H{
			"name": "   ",
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("create without auth", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases", gin.H{
			"name": "Test",
		}, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("list without auth", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/knowledge-bases", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("get kb route exists", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/knowledge-bases/kb_nonexist", nil, headers)
		if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 404 or 500", w.Code)
		}
	})

	t.Run("patch no fields", func(t *testing.T) {
		w := doJSON(r, http.MethodPatch, "/api/v1/knowledge-bases/kb_test", gin.H{}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("delete kb route", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/api/v1/knowledge-bases/kb_nonexist", nil, headers)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 200 or 500", w.Code)
		}
	})
}

// ---------- Document E2E ----------

func TestE2E_DocumentRoutes(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")
	headers := map[string]string{"Authorization": auth}

	t.Run("upload without file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/kb_test/documents", nil)
		req.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("upload unsupported type", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.pdf")
		part.Write([]byte("pdf content"))
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/kb_test/documents", body)
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("list documents route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/knowledge-bases/kb_test/documents", nil, headers)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 200 or 500", w.Code)
		}
	})

	t.Run("get document route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/knowledge-bases/kb_test/documents/doc_test", nil, headers)
		if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 404 or 500", w.Code)
		}
	})

	t.Run("delete document route", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/api/v1/knowledge-bases/kb_test/documents/doc_test", nil, headers)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 200 or 500", w.Code)
		}
	})

	t.Run("reindex document route", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/knowledge-bases/kb_test/documents/doc_test/reindex", nil, headers)
		if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 404 or 500", w.Code)
		}
	})
}

// ---------- Task E2E ----------

func TestE2E_TaskRoutes(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")
	headers := map[string]string{"Authorization": auth}

	t.Run("get task route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/tasks/task_nonexist", nil, headers)
		if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 404 or 500", w.Code)
		}
	})

	t.Run("get task without auth", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/tasks/task_test", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})
}

// ---------- Session E2E ----------

func TestE2E_SessionCRUD(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")
	headers := map[string]string{"Authorization": auth}

	t.Run("create session empty kb_id", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/sessions", gin.H{
			"kb_id": "",
			"title": "test",
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("create session empty title", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/sessions", gin.H{
			"kb_id": "kb_test",
			"title": "  ",
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("create session without auth", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/sessions", gin.H{
			"kb_id": "kb_test",
			"title": "test",
		}, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("list sessions route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/sessions", nil, headers)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 200 or 500", w.Code)
		}
	})

	t.Run("get session route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/sessions/sess_nonexist", nil, headers)
		if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 404 or 500", w.Code)
		}
	})

	t.Run("get session messages route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/sessions/sess_test/messages", nil, headers)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 200 or 500", w.Code)
		}
	})

	t.Run("delete session route", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/api/v1/sessions/sess_nonexist", nil, headers)
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 200 or 500", w.Code)
		}
	})
}

// ---------- Chat E2E ----------

func TestE2E_ChatCompletions(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")
	headers := map[string]string{"Authorization": auth}

	t.Run("empty kb_id", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/chat/completions", gin.H{
			"kb_id":    "",
			"question": "what?",
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("empty question", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/chat/completions", gin.H{
			"kb_id":    "kb_test",
			"question": "",
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("question too long", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/chat/completions", gin.H{
			"kb_id":    "kb_test",
			"question": strings.Repeat("a", 2001),
		}, headers)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("without auth", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/chat/completions", gin.H{
			"kb_id":    "kb_test",
			"question": "hello",
		}, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("non-json body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader("{bad"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})
}

// ---------- Trace E2E ----------

func TestE2E_TraceRoutes(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")
	headers := map[string]string{"Authorization": auth}

	t.Run("get trace route", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/chat/traces/trace_nonexist", nil, headers)
		if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 404 or 500", w.Code)
		}
	})

	t.Run("get trace without auth", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/chat/traces/trace_test", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnauthorized)
		}
	})
}

// ---------- Request ID E2E ----------

func TestE2E_RequestIDPropagation(t *testing.T) {
	t.Parallel()
	r, _ := newE2EServer()

	t.Run("custom request id echoed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		req.Header.Set("X-Request-Id", "req_custom_123")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if got := w.Header().Get("X-Request-Id"); got != "req_custom_123" {
			t.Fatalf("X-Request-Id = %q; want req_custom_123", got)
		}
	})

	t.Run("auto request id generated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if got := w.Header().Get("X-Request-Id"); got == "" {
			t.Fatalf("X-Request-Id should be auto-generated")
		}
	})
}

// ---------- CORS E2E ----------

func TestE2E_CORS(t *testing.T) {
	t.Parallel()
	r, _ := newE2EServer()

	t.Run("options returns 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/healthz", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusNoContent)
		}
	})
}

// ---------- Response Format E2E ----------

func TestE2E_ResponseFormat(t *testing.T) {
	t.Parallel()
	r, _ := newE2EServer()

	t.Run("success response structure", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/api/v1/healthz", nil, nil)
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)

		if _, ok := resp["data"]; !ok {
			t.Fatalf("response missing 'data' field")
		}
		if _, ok := resp["request_id"]; !ok {
			t.Fatalf("response missing 'request_id' field")
		}
		if _, ok := resp["timestamp"]; !ok {
			t.Fatalf("response missing 'timestamp' field")
		}
	})

	t.Run("error response structure", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/api/v1/auth/login", gin.H{
			"username": "",
			"password": "",
		}, nil)
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)

		errObj, ok := resp["error"].(map[string]interface{})
		if !ok {
			t.Fatalf("response missing 'error' field")
		}
		if _, ok := errObj["code"]; !ok {
			t.Fatalf("error missing 'code' field")
		}
		if _, ok := errObj["message"]; !ok {
			t.Fatalf("error missing 'message' field")
		}
	})
}

// ---------- Multi-file Upload E2E ----------

func TestE2E_MultiFileUpload(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")

	t.Run("multi-file upload route", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for i := 0; i < 3; i++ {
			part, _ := writer.CreateFormFile("files", "file"+string(rune('0'+i))+".txt")
			part.Write([]byte("content " + string(rune('0'+i))))
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/kb_test/documents", body)
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 201 or 500", w.Code)
		}
	})

	t.Run("too many files", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for i := 0; i < 21; i++ {
			part, _ := writer.CreateFormFile("files", "file.txt")
			part.Write([]byte("content"))
		}
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/kb_test/documents", body)
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusUnprocessableEntity)
		}
	})
}

// ---------- All Routes E2E ----------

func TestE2E_AllMethodsCovered(t *testing.T) {
	t.Parallel()
	r, app := newE2EServer()
	auth := e2eAuthHeader(t, app, "u_test")

	routes := []struct {
		method string
		path   string
		desc   string
	}{
		{http.MethodPost, "/api/v1/auth/login", "login"},
		{http.MethodGet, "/api/v1/healthz", "healthz"},
		{http.MethodGet, "/api/v1/readyz", "readyz"},
		{http.MethodGet, "/api/v1/metrics", "metrics"},
		{http.MethodGet, "/api/v1/auth/me", "auth me"},
		{http.MethodPost, "/api/v1/knowledge-bases", "create kb"},
		{http.MethodGet, "/api/v1/knowledge-bases", "list kbs"},
		{http.MethodGet, "/api/v1/knowledge-bases/kb_test", "get kb"},
		{http.MethodPatch, "/api/v1/knowledge-bases/kb_test", "patch kb"},
		{http.MethodDelete, "/api/v1/knowledge-bases/kb_test", "delete kb"},
		{http.MethodGet, "/api/v1/knowledge-bases/kb_test/documents", "list docs"},
		{http.MethodGet, "/api/v1/knowledge-bases/kb_test/documents/doc_test", "get doc"},
		{http.MethodDelete, "/api/v1/knowledge-bases/kb_test/documents/doc_test", "delete doc"},
		{http.MethodPost, "/api/v1/knowledge-bases/kb_test/documents/doc_test/reindex", "reindex"},
		{http.MethodGet, "/api/v1/tasks/task_test", "get task"},
		{http.MethodPost, "/api/v1/sessions", "create session"},
		{http.MethodGet, "/api/v1/sessions", "list sessions"},
		{http.MethodGet, "/api/v1/sessions/sess_test", "get session"},
		{http.MethodGet, "/api/v1/sessions/sess_test/messages", "list messages"},
		{http.MethodDelete, "/api/v1/sessions/sess_test", "delete session"},
		{http.MethodPost, "/api/v1/chat/completions", "chat"},
		{http.MethodGet, "/api/v1/chat/traces/trace_test", "get trace"},
	}

	for _, rt := range routes {
		t.Run(rt.desc, func(t *testing.T) {
			var req *http.Request
			if rt.method == http.MethodPost || rt.method == http.MethodPatch {
				req = httptest.NewRequest(rt.method, rt.path, bytes.NewBufferString(`{}`))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(rt.method, rt.path, nil)
			}
			if rt.path != "/api/v1/healthz" && rt.path != "/api/v1/readyz" && rt.path != "/api/v1/metrics" && rt.path != "/api/v1/auth/login" {
				req.Header.Set("Authorization", auth)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code == http.StatusMethodNotAllowed {
				t.Fatalf("route %s %s returned 405 MethodNotAllowed", rt.method, rt.path)
			}
		})
	}
}
