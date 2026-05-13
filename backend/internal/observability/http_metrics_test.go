package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPMetricsMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(HTTPMetrics())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/fail", func(c *gin.Context) {
		c.String(http.StatusInternalServerError, "fail")
	})

	t.Run("records successful request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("records failed request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/fail", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestPrometheusHandler(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Reset metrics for clean test
	metrics = &httpMetricsRegistry{
		data: make(map[metricKey]metricAggregate),
	}

	r := gin.New()
	r.Use(HTTPMetrics())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/metrics", PrometheusHandler())

	// Make some requests to generate metrics
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
	}

	// Get metrics
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "memobase_http_requests_total") {
		t.Fatalf("metrics missing memobase_http_requests_total")
	}
	if !strings.Contains(body, "memobase_http_request_duration_seconds") {
		t.Fatalf("metrics missing memobase_http_request_duration_seconds")
	}
	if !strings.Contains(body, "memobase_up 1") {
		t.Fatalf("metrics missing memobase_up 1")
	}
	if !strings.Contains(body, "memobase_http_requests_in_flight") {
		t.Fatalf("metrics missing memobase_http_requests_in_flight")
	}
	if !strings.Contains(body, "method=\"GET\"") {
		t.Fatalf("metrics missing method label")
	}
	if !strings.Contains(body, "route=\"/test\"") {
		t.Fatalf("metrics missing route label")
	}
	if !strings.Contains(body, "status=\"200\"") {
		t.Fatalf("metrics missing status label")
	}
}

func TestPrometheusHandlerContentType(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/metrics", PrometheusHandler())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "text/plain; version=0.0.4" {
		t.Fatalf("Content-Type = %q; want %q", ct, "text/plain; version=0.0.4")
	}
}

func TestMetricsRegistryObserve(t *testing.T) {
	t.Parallel()

	reg := &httpMetricsRegistry{
		data: make(map[metricKey]metricAggregate),
	}

	reg.observe("GET", "/api", "200", 0.1)
	reg.observe("GET", "/api", "200", 0.2)
	reg.observe("POST", "/api", "201", 0.3)

	key1 := metricKey{Method: "GET", Route: "/api", Status: "200"}
	if reg.data[key1].Count != 2 {
		t.Fatalf("count = %d; want 2", reg.data[key1].Count)
	}
	if reg.data[key1].DurationSum < 0.29 || reg.data[key1].DurationSum > 0.31 {
		t.Fatalf("duration sum = %f; want ~0.3", reg.data[key1].DurationSum)
	}

	key2 := metricKey{Method: "POST", Route: "/api", Status: "201"}
	if reg.data[key2].Count != 1 {
		t.Fatalf("count = %d; want 1", reg.data[key2].Count)
	}
}

func TestPrometheusHTTPHandler(t *testing.T) {
	t.Parallel()

	handler := PrometheusHTTPHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; version=0.0.4" {
		t.Fatalf("Content-Type = %q; want %q", ct, "text/plain; version=0.0.4")
	}
	if !strings.Contains(w.Body.String(), "memobase_up") {
		t.Fatalf("body missing memobase_up")
	}
}
