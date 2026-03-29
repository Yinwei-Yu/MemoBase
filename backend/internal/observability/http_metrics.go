package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type metricKey struct {
	Method string
	Route  string
	Status string
}

type metricAggregate struct {
	Count       uint64
	DurationSum float64
}

type httpMetricsRegistry struct {
	inFlight atomic.Int64
	mu       sync.RWMutex
	data     map[metricKey]metricAggregate
}

var metrics = &httpMetricsRegistry{
	data: make(map[metricKey]metricAggregate),
}

func HTTPMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		metrics.inFlight.Add(1)
		defer metrics.inFlight.Add(-1)

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())
		elapsed := time.Since(start).Seconds()
		metrics.observe(method, route, status, elapsed)
	}
}

func PrometheusHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4")
		_, _ = c.Writer.WriteString(metrics.render())
	}
}

func (r *httpMetricsRegistry) observe(method, route, status string, durationSeconds float64) {
	key := metricKey{Method: method, Route: route, Status: status}
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.data[key]
	current.Count++
	current.DurationSum += durationSeconds
	r.data[key] = current
}

func (r *httpMetricsRegistry) render() string {
	var sb strings.Builder
	sb.WriteString("# HELP memobase_http_requests_total Total number of HTTP requests.\n")
	sb.WriteString("# TYPE memobase_http_requests_total counter\n")

	r.mu.RLock()
	keys := make([]metricKey, 0, len(r.data))
	for key := range r.data {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		if keys[i].Route != keys[j].Route {
			return keys[i].Route < keys[j].Route
		}
		return keys[i].Status < keys[j].Status
	})
	for _, key := range keys {
		aggregate := r.data[key]
		sb.WriteString(fmt.Sprintf(
			"memobase_http_requests_total{method=%q,route=%q,status=%q} %d\n",
			key.Method,
			key.Route,
			key.Status,
			aggregate.Count,
		))
	}
	sb.WriteString("# HELP memobase_http_request_duration_seconds Request duration summary in seconds.\n")
	sb.WriteString("# TYPE memobase_http_request_duration_seconds summary\n")
	for _, key := range keys {
		aggregate := r.data[key]
		sb.WriteString(fmt.Sprintf(
			"memobase_http_request_duration_seconds_sum{method=%q,route=%q,status=%q} %.6f\n",
			key.Method,
			key.Route,
			key.Status,
			aggregate.DurationSum,
		))
		sb.WriteString(fmt.Sprintf(
			"memobase_http_request_duration_seconds_count{method=%q,route=%q,status=%q} %d\n",
			key.Method,
			key.Route,
			key.Status,
			aggregate.Count,
		))
	}
	r.mu.RUnlock()

	sb.WriteString("# HELP memobase_http_requests_in_flight Number of in-flight HTTP requests.\n")
	sb.WriteString("# TYPE memobase_http_requests_in_flight gauge\n")
	sb.WriteString(fmt.Sprintf("memobase_http_requests_in_flight %d\n", r.inFlight.Load()))
	sb.WriteString("# HELP memobase_up Service liveness state.\n")
	sb.WriteString("# TYPE memobase_up gauge\n")
	sb.WriteString(fmt.Sprintf("memobase_up %d\n", 1))

	return sb.String()
}

func PrometheusHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(metrics.render()))
	}
}
