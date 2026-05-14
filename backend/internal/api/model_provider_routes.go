package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"memobase/backend/internal/core"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&modelProviderRegistrar{})
}

type modelProviderRegistrar struct{}

func (modelProviderRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.GET("/model-providers", handleListProviders(app))
	authed.POST("/model-providers", handleCreateProvider(app))
	authed.PATCH("/model-providers/:id", handleUpdateProvider(app))
	authed.DELETE("/model-providers/:id", handleDeleteProvider(app))
	authed.POST("/model-providers/:id/test", handleTestProvider(app))
}

func handleListProviders(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := userIDFrom(c)
		providers, err := app.Store.ListModelProviders(c.Request.Context(), userID)
		if err != nil {
			util.Internal(c, "failed to list providers")
			return
		}
		// Strip raw API key from response, only return masked
		for i := range providers {
			providers[i].APIKey = ""
		}
		util.Success(c, http.StatusOK, providers)
	}
}

func handleCreateProvider(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name         string `json:"name"`
			ProviderType string `json:"provider_type"`
			APIBaseURL   string `json:"api_base_url"`
			APIKey       string `json:"api_key"`
			DefaultModel string `json:"default_model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" || utf8.RuneCountInString(name) > 64 {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "name is required (max 64 chars)", nil)
			return
		}
		apiBaseURL := strings.TrimSpace(req.APIBaseURL)
		if apiBaseURL == "" {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "api_base_url is required", nil)
			return
		}
		providerType := strings.TrimSpace(req.ProviderType)
		if providerType == "" {
			providerType = "openai_compatible"
		}
		defaultModel := strings.TrimSpace(req.DefaultModel)
		if defaultModel == "" {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "default_model is required", nil)
			return
		}

		userID := userIDFrom(c)
		mp, err := app.Store.CreateModelProvider(c.Request.Context(), userID, name, providerType, apiBaseURL, req.APIKey, defaultModel)
		if err != nil {
			util.Internal(c, "failed to create provider")
			return
		}
		mp.APIKey = ""
		util.Success(c, http.StatusCreated, mp)
	}
}

func handleUpdateProvider(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerID := c.Param("id")
		var req struct {
			Name         *string `json:"name"`
			APIBaseURL   *string `json:"api_base_url"`
			APIKey       *string `json:"api_key"`
			DefaultModel *string `json:"default_model"`
			IsDefault    *bool   `json:"is_default"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}
		if req.Name != nil {
			trimmed := strings.TrimSpace(*req.Name)
			if trimmed == "" || utf8.RuneCountInString(trimmed) > 64 {
				util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "name must be 1-64 chars", nil)
				return
			}
			req.Name = &trimmed
		}
		if req.APIBaseURL != nil {
			trimmed := strings.TrimSpace(*req.APIBaseURL)
			if trimmed == "" {
				util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "api_base_url cannot be empty", nil)
				return
			}
			req.APIBaseURL = &trimmed
		}
		if req.DefaultModel != nil {
			trimmed := strings.TrimSpace(*req.DefaultModel)
			if trimmed == "" {
				util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "default_model cannot be empty", nil)
				return
			}
			req.DefaultModel = &trimmed
		}

		userID := userIDFrom(c)
		mp, err := app.Store.UpdateModelProvider(c.Request.Context(), userID, providerID, req.Name, req.APIBaseURL, req.APIKey, req.DefaultModel, req.IsDefault)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "NOT_FOUND", "provider not found", nil)
				return
			}
			util.Internal(c, "failed to update provider")
			return
		}
		mp.APIKey = ""
		util.Success(c, http.StatusOK, mp)
	}
}

func handleDeleteProvider(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := userIDFrom(c)
		providerID := c.Param("id")
		if err := app.Store.DeleteModelProvider(c.Request.Context(), userID, providerID); err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "NOT_FOUND", "provider not found", nil)
				return
			}
			util.Internal(c, "failed to delete provider")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	}
}

func handleTestProvider(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := userIDFrom(c)
		providerID := c.Param("id")

		mp, err := app.Store.GetModelProviderRaw(c.Request.Context(), providerID)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "NOT_FOUND", "provider not found", nil)
				return
			}
			util.Internal(c, "failed to get provider")
			return
		}
		if mp.UserID != userID {
			util.Fail(c, http.StatusNotFound, "NOT_FOUND", "provider not found", nil)
			return
		}

		answer, latency, err := app.Provider.TestConnection(c.Request.Context(), mp.APIBaseURL, mp.APIKey, mp.DefaultModel)
		if err != nil {
			util.Success(c, http.StatusOK, gin.H{
				"success":      false,
				"error":        err.Error(),
				"latency_ms":   latency.Milliseconds(),
				"model":        mp.DefaultModel,
			})
			return
		}

		util.Success(c, http.StatusOK, gin.H{
			"success":    true,
			"answer":     answer,
			"latency_ms": latency.Milliseconds(),
			"model":      mp.DefaultModel,
		})
	}
}
