package api

import (
	"memobase/backend/internal/core"

	"github.com/gin-gonic/gin"
)

// Registrar is implemented by each domain module to register its routes.
type Registrar interface {
	Register(public *gin.RouterGroup, authed *gin.RouterGroup, app *core.App)
}

var registry []Registrar

// Register adds a route registrar to the global registry.
// Call this from init() in each domain route file.
func Register(r Registrar) {
	registry = append(registry, r)
}

// RegisterAll registers all domain routes on the given engine.
func RegisterAll(r *gin.Engine, app *core.App) {
	public := r.Group("/api/v1")
	authed := public.Group("/")
	authed.Use(AuthRequired(app))

	for _, reg := range registry {
		reg.Register(public, authed, app)
	}
}
