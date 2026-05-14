package api

import (
	"context"
	"net/http"
	"time"

	"memobase/backend/internal/core"
	"memobase/backend/internal/infra"
	"memobase/backend/internal/observability"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&healthRegistrar{})
}

type healthRegistrar struct{}

func (healthRegistrar) Register(public *gin.RouterGroup, _ *gin.RouterGroup, app *core.App) {
	public.GET("/healthz", handleHealthz())
	public.GET("/readyz", handleReadyz(app))
	public.GET("/metrics", observability.PrometheusHandler())
		public.GET("/metrics/summary", handleMetricsSummary())
	}

	func handleMetricsSummary() gin.HandlerFunc {
		return func(c *gin.Context) {
			util.Success(c, http.StatusOK, observability.Snapshot())
		}
	}

func handleHealthz() gin.HandlerFunc {
	return func(c *gin.Context) {
		util.Success(c, http.StatusOK, gin.H{"status": "ok"})
	}
}

func handleReadyz(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		checks := map[string]string{}
		status := "ready"
		if err := infra.Ping(ctx, app.DB); err != nil {
			checks["db"] = "down"
			status = "not_ready"
		} else {
			checks["db"] = "up"
		}
		if app.Agent != nil {
			healthCtx, healthCancel := context.WithTimeout(ctx, 2*time.Second)
			resp, err := app.Agent.HealthCheck(healthCtx)
			healthCancel()
			if err != nil {
				checks["agent_service"] = "down"
				status = "not_ready"
			} else {
				checks["agent_service"] = resp.Status
				// Propagate sub-checks from agent service (qdrant, ollama, etc.)
				for k, v := range resp.Checks {
					checks[k] = v
				}
			}
		} else {
			checks["agent_service"] = "disabled"
			status = "not_ready"
		}
		checks["storage"] = "up"
		if status != "ready" {
			util.Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "dependency not ready", gin.H{"checks": checks})
			return
		}
		util.Success(c, http.StatusOK, gin.H{"status": status, "checks": checks})
	}
}
