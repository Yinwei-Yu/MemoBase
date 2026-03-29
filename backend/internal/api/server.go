package api

import (
	"log/slog"
	"os"

	"memobase/backend/internal/core"
	"memobase/backend/internal/observability"

	"github.com/gin-gonic/gin"
)

func NewServer(app *core.App) *gin.Engine {
	if app.Config.AppEnv != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(Cors(app.Config.CORSOrigin))
	r.Use(RequestID())
	r.Use(observability.HTTPMetrics())
	r.Use(Logger(app.Logger))
	RegisterRoutes(r, app)
	return r
}

func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "dev" {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h).With(
		slog.String("service", "memobase-backend"),
		slog.String("env", env),
	)
}
