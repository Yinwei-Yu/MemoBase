package api

import (
	"net/http"
	"strings"

	"memobase/backend/internal/core"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&sessionRegistrar{})
}

type sessionRegistrar struct{}

func (sessionRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.POST("/sessions", handleCreateSession(app))
	authed.GET("/sessions", handleListSessions(app))
	authed.GET("/sessions/:session_id", handleGetSession(app))
	authed.GET("/sessions/:session_id/messages", handleListMessages(app))
	authed.DELETE("/sessions/:session_id", handleDeleteSession(app))
}

func handleCreateSession(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			KBID  string `json:"kb_id"`
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}
		if strings.TrimSpace(req.KBID) == "" || strings.TrimSpace(req.Title) == "" {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "kb_id and title are required", nil)
			return
		}
		s, err := app.Store.CreateSession(c.Request.Context(), userIDFrom(c), req.KBID, req.Title)
		if err != nil {
			util.Internal(c, "failed to create session")
			return
		}
		util.Success(c, http.StatusCreated, s)
	}
}

func handleListSessions(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListSessions(c.Request.Context(), userIDFrom(c), c.Query("kb_id"), pageSize, offset)
		if err != nil {
			util.Internal(c, "failed to list sessions")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"items": items, "pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total}})
	}
}

func handleGetSession(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, err := app.Store.GetSession(c.Request.Context(), userIDFrom(c), c.Param("session_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found", nil)
				return
			}
			util.Internal(c, "failed to get session")
			return
		}
		util.Success(c, http.StatusOK, s)
	}
}

func handleListMessages(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := app.Store.GetSession(c.Request.Context(), userIDFrom(c), c.Param("session_id")); err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found", nil)
				return
			}
			util.Internal(c, "failed to get session")
			return
		}
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListMessages(c.Request.Context(), c.Param("session_id"), pageSize, offset)
		if err != nil {
			util.Internal(c, "failed to list messages")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"items": items, "pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total}})
	}
}

func handleDeleteSession(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := app.Store.DeleteSession(c.Request.Context(), userIDFrom(c), c.Param("session_id")); err != nil {
			util.Internal(c, "failed to delete session")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	}
}
