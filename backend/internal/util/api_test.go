package util

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	return c, w
}

func TestRequestIDFallback(t *testing.T) {
	t.Parallel()
	c, _ := newTestContext()
	if got := RequestID(c); got != "req_unknown" {
		t.Fatalf("RequestID() = %q; want %q", got, "req_unknown")
	}
}

func TestSuccessEnvelope(t *testing.T) {
	t.Parallel()
	c, w := newTestContext()
	c.Set("request_id", "req_1")

	Success(c, http.StatusCreated, gin.H{"ok": true})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusCreated)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if body["request_id"] != "req_1" {
		t.Fatalf("request_id = %v; want req_1", body["request_id"])
	}
	if body["timestamp"] == "" {
		t.Fatalf("timestamp should not be empty")
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok || data["ok"] != true {
		t.Fatalf("data envelope malformed: %#v", body["data"])
	}
}

func TestFailEnvelope(t *testing.T) {
	t.Parallel()
	c, w := newTestContext()
	c.Set("request_id", "req_2")

	Fail(c, http.StatusBadRequest, "BAD", "bad input", gin.H{"field": "name"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusBadRequest)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error envelope malformed: %#v", body["error"])
	}
	if errObj["code"] != "BAD" || errObj["message"] != "bad input" {
		t.Fatalf("error object = %#v; want code=BAD/message=bad input", errObj)
	}
}
