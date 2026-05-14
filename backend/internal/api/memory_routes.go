package api

import (
	"net/http"
	"strconv"

	"memobase/backend/internal/core"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&memoryRegistrar{})
}

type memoryRegistrar struct{}

func (memoryRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.GET("/memories", handleListMemories(app))
	authed.GET("/memories/:memory_id", handleGetMemory(app))
	authed.PATCH("/memories/:memory_id", handlePatchMemory(app))
	authed.DELETE("/memories/:memory_id", handleDeleteMemory(app))
	authed.POST("/memories/consolidate", handleConsolidateMemories(app))
}

func handleListMemories(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := userIDFrom(c)
		memType := c.Query("type")
		limit := 50
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		memories, err := app.Store.ListUserMemories(c.Request.Context(), userID, memType, limit)
		if err != nil {
			util.Internal(c, "failed to list memories")
			return
		}

		util.Success(c, http.StatusOK, gin.H{"memories": memories})
	}
}

func handleGetMemory(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		memoryID := c.Param("memory_id")
		userID := userIDFrom(c)

		mem, err := app.Store.GetMemory(c.Request.Context(), memoryID)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "NOT_FOUND", "memory not found", nil)
			} else {
				util.Internal(c, "failed to get memory")
			}
			return
		}

		// Verify ownership
		if mem.UserID == nil || *mem.UserID != userID {
			util.Fail(c, http.StatusNotFound, "NOT_FOUND", "memory not found", nil)
			return
		}

		util.Success(c, http.StatusOK, mem)
	}
}

func handlePatchMemory(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		memoryID := c.Param("memory_id")
		userID := userIDFrom(c)

		// Verify ownership
		mem, err := app.Store.GetMemory(c.Request.Context(), memoryID)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "NOT_FOUND", "memory not found", nil)
			} else {
				util.Internal(c, "failed to get memory")
			}
			return
		}
		if mem.UserID == nil || *mem.UserID != userID {
			util.Fail(c, http.StatusNotFound, "NOT_FOUND", "memory not found", nil)
			return
		}

		var req struct {
			Summary    *string  `json:"summary"`
			Importance *float64 `json:"importance"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}

		if req.Summary != nil {
			// Update summary via raw SQL since we don't have a dedicated method
			_, err := app.Store.DB.ExecContext(c.Request.Context(),
				`UPDATE memories SET summary=$2 WHERE id=$1`, memoryID, *req.Summary)
			if err != nil {
				util.Internal(c, "failed to update summary")
				return
			}
		}
		if req.Importance != nil {
			if err := app.Store.UpdateMemoryImportance(c.Request.Context(), memoryID, *req.Importance); err != nil {
				util.Internal(c, "failed to update importance")
				return
			}
		}

		// Return updated memory
		updated, _ := app.Store.GetMemory(c.Request.Context(), memoryID)
		util.Success(c, http.StatusOK, updated)
	}
}

func handleDeleteMemory(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		memoryID := c.Param("memory_id")
		userID := userIDFrom(c)

		// Verify ownership
		mem, err := app.Store.GetMemory(c.Request.Context(), memoryID)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "NOT_FOUND", "memory not found", nil)
			} else {
				util.Internal(c, "failed to get memory")
			}
			return
		}
		if mem.UserID == nil || *mem.UserID != userID {
			util.Fail(c, http.StatusNotFound, "NOT_FOUND", "memory not found", nil)
			return
		}

		_, err = app.Store.DB.ExecContext(c.Request.Context(),
			`DELETE FROM memories WHERE id=$1`, memoryID)
		if err != nil {
			util.Internal(c, "failed to delete memory")
			return
		}

		util.Success(c, http.StatusOK, gin.H{"deleted": memoryID})
	}
}

func handleConsolidateMemories(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := userIDFrom(c)

		mc := core.NewMemoryConsolidator(app)
		if err := mc.Consolidate(c.Request.Context(), userID); err != nil {
			util.Internal(c, "consolidation failed: "+err.Error())
			return
		}

		util.Success(c, http.StatusOK, gin.H{"status": "consolidated"})
	}
}
