package api

import (
	"net/http"

	"memobase/backend/internal/core"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&taskRegistrar{})
}

type taskRegistrar struct{}

func (taskRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.GET("/tasks/:task_id", handleGetTask(app))
}

func handleGetTask(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		task, err := app.Store.GetTask(c.Request.Context(), c.Param("task_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "TASK_NOT_FOUND", "task not found", nil)
				return
			}
			util.Internal(c, "failed to get task")
			return
		}
		util.Success(c, http.StatusOK, task)
	}
}
