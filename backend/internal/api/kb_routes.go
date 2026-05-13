package api

import (
	"net/http"

	"memobase/backend/internal/core"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&kbRegistrar{})
}

type kbRegistrar struct{}

func (kbRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.POST("/knowledge-bases", handleCreateKB(app))
	authed.GET("/knowledge-bases", handleListKB(app))
	authed.GET("/knowledge-bases/:kb_id", handleGetKB(app))
	authed.PATCH("/knowledge-bases/:kb_id", handlePatchKB(app))
	authed.DELETE("/knowledge-bases/:kb_id", handleDeleteKB(app))
}

func handleCreateKB(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}
		name := req.Name
		description := req.Description
		tags := req.Tags
		if err := validateKBFields(&name, &description, &tags); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		kb, err := app.Store.CreateKB(c.Request.Context(), userIDFrom(c), name, description, tags)
		if err != nil {
			util.Internal(c, "failed to create knowledge base")
			return
		}
		util.Success(c, http.StatusCreated, kb)
	}
}

func handleListKB(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListKB(c.Request.Context(), userIDFrom(c), pageSize, offset, c.Query("keyword"))
		if err != nil {
			util.Internal(c, "failed to list knowledge bases")
			return
		}
		util.Success(c, http.StatusOK, gin.H{
			"items":      items,
			"pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total},
		})
	}
}

func handleGetKB(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kb, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), c.Param("kb_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
				return
			}
			util.Internal(c, "failed to get knowledge base")
			return
		}
		util.Success(c, http.StatusOK, kb)
	}
}

func handlePatchKB(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name        *string   `json:"name"`
			Description *string   `json:"description"`
			Tags        *[]string `json:"tags"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}
		if req.Name == nil && req.Description == nil && req.Tags == nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "at least one field is required", nil)
			return
		}
		if err := validateKBFields(req.Name, req.Description, req.Tags); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		kb, err := app.Store.PatchKB(c.Request.Context(), userIDFrom(c), c.Param("kb_id"), req.Name, req.Description, req.Tags)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
				return
			}
			util.Internal(c, "failed to update knowledge base")
			return
		}
		util.Success(c, http.StatusOK, kb)
	}
}

func handleDeleteKB(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if err := app.Store.DeleteKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Internal(c, "failed to delete knowledge base")
			return
		}
		_ = app.Qdrant.DeleteCollection(c.Request.Context(), app.QdrantCollectionForKB(kbID))
		app.InvalidateBM25Index(kbID)
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	}
}
