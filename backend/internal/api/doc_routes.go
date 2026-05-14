package api

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"memobase/backend/internal/core"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&docRegistrar{})
}

type docRegistrar struct{}

func (docRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.POST("/knowledge-bases/:kb_id/documents", handleUploadDocuments(app))
	authed.GET("/knowledge-bases/:kb_id/documents", handleListDocuments(app))
	authed.GET("/knowledge-bases/:kb_id/documents/:doc_id", handleGetDocument(app))
	authed.GET("/knowledge-bases/:kb_id/documents/:doc_id/content", handleGetDocumentContent(app))
	authed.DELETE("/knowledge-bases/:kb_id/documents/:doc_id", handleDeleteDocument(app))
	authed.POST("/knowledge-bases/:kb_id/documents/:doc_id/reindex", handleReindexDocument(app))
}

func handleUploadDocuments(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if _, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			return
		}
		form, err := c.MultipartForm()
		if err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "file is required", nil)
			return
		}

		fileHeaders := make([]*multipart.FileHeader, 0, len(form.File["files"])+len(form.File["file"]))
		fileHeaders = append(fileHeaders, form.File["files"]...)
		fileHeaders = append(fileHeaders, form.File["file"]...)
		if len(fileHeaders) == 0 {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "at least one file is required", nil)
			return
		}
		if len(fileHeaders) > maxUploadFileCount {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", fmt.Sprintf("too many files (max %d)", maxUploadFileCount), nil)
			return
		}
		for _, fileHeader := range fileHeaders {
			if fileHeader.Size > maxUploadFileSize {
				util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "file too large (max 20MB each)", nil)
				return
			}
			ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
			if !isSupportedUploadExt(ext) {
				util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "unsupported file type: only .txt and .md are currently supported", nil)
				return
			}
		}

		title := strings.TrimSpace(c.PostForm("title"))
		chunkSize, _ := strconv.Atoi(c.DefaultPostForm("chunk_size", "500"))
		overlap, _ := strconv.Atoi(c.DefaultPostForm("chunk_overlap", "100"))
		if chunkSize < 200 || chunkSize > 1200 {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "chunk_size must be between 200 and 1200", nil)
			return
		}
		if overlap < 0 || overlap > 300 {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "chunk_overlap must be between 0 and 300", nil)
			return
		}
		items := make([]uploadDocumentItem, 0, len(fileHeaders))
		for _, fileHeader := range fileHeaders {
			docTitle := title
			if docTitle == "" || len(fileHeaders) > 1 {
				docTitle = fileHeader.Filename
			}

			file, err := fileHeader.Open()
			if err != nil {
				util.Internal(c, "failed to open uploaded file")
				return
			}

			doc, err := app.Store.CreateDocument(c.Request.Context(), kbID, docTitle, fileHeader.Filename)
			if err != nil {
				_ = file.Close()
				util.Internal(c, "failed to create document")
				return
			}

			path, err := core.SaveUploadedFile(app.Config.StorageDir, kbID, doc.ID, fileHeader.Filename, file)
			_ = file.Close()
			if err != nil {
				_ = app.Store.DeleteDocument(c.Request.Context(), kbID, doc.ID)
				util.Internal(c, "failed to save uploaded file")
				return
			}

			task, err := app.Store.CreateTask(c.Request.Context(), "document_index", map[string]interface{}{"doc_id": doc.ID, "kb_id": kbID})
			if err != nil {
				_ = os.Remove(path)
				_ = app.Store.DeleteDocument(c.Request.Context(), kbID, doc.ID)
				util.Internal(c, "failed to create task")
				return
			}

			go app.ProcessDocument(task.ID, kbID, doc.ID, path, chunkSize, overlap)
			items = append(items, uploadDocumentItem{
				DocID:     doc.ID,
				KBID:      kbID,
				FileName:  doc.FileName,
				Status:    doc.Status,
				TaskID:    task.ID,
				CreatedAt: doc.CreatedAt,
			})
		}

		util.Success(c, http.StatusCreated, uploadDocumentsResponse{
			Items:         items,
			UploadedCount: len(items),
		})
	}
}

func handleListDocuments(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if _, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			return
		}
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListDocuments(c.Request.Context(), kbID, c.Query("status"), pageSize, offset)
		if err != nil {
			util.Internal(c, "failed to list documents")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"items": items, "pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total}})
	}
}

func handleGetDocument(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if _, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			return
		}
		doc, err := app.Store.GetDocument(c.Request.Context(), kbID, c.Param("doc_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "DOC_NOT_FOUND", "document not found", nil)
				return
			}
			util.Internal(c, "failed to get document")
			return
		}
		util.Success(c, http.StatusOK, doc)
	}
}

func handleGetDocumentContent(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if _, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			return
		}
		doc, err := app.Store.GetDocumentContent(c.Request.Context(), kbID, c.Param("doc_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "DOC_NOT_FOUND", "document not found", nil)
				return
			}
			util.Internal(c, "failed to get document content")
			return
		}
		util.Success(c, http.StatusOK, doc)
	}
}

func handleDeleteDocument(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		docID := c.Param("doc_id")
		if _, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			return
		}
		if err := app.Store.DeleteDocument(c.Request.Context(), kbID, docID); err != nil {
			util.Internal(c, "failed to delete document")
			return
		}
		if app.Agent != nil {
			_ = app.Agent.DeleteDocumentVectors(c.Request.Context(), app.QdrantCollectionForKB(kbID), docID)
		}
		app.InvalidateBM25Index(kbID)
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	}
}

func handleReindexDocument(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if _, err := app.Store.GetKB(c.Request.Context(), userIDFrom(c), kbID); err != nil {
			util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			return
		}
		doc, err := app.Store.GetDocument(c.Request.Context(), kbID, c.Param("doc_id"))
		if err != nil {
			util.Fail(c, http.StatusNotFound, "DOC_NOT_FOUND", "document not found", nil)
			return
		}
		chunkSize, _ := strconv.Atoi(c.DefaultQuery("chunk_size", "500"))
		overlap, _ := strconv.Atoi(c.DefaultQuery("chunk_overlap", "100"))
		if chunkSize < 200 || chunkSize > 1200 {
			chunkSize = 500
		}
		if overlap < 0 || overlap > 300 {
			overlap = 100
		}
		filePath := filepath.Join(app.Config.StorageDir, kbID, doc.ID+"_"+filepath.Base(doc.FileName))
		task, err := app.Store.CreateTask(c.Request.Context(), "document_reindex", map[string]interface{}{"doc_id": doc.ID, "kb_id": doc.KBID})
		if err != nil {
			util.Internal(c, "failed to create task")
			return
		}
		go app.ProcessDocument(task.ID, doc.KBID, doc.ID, filePath, chunkSize, overlap)
		util.Success(c, http.StatusOK, gin.H{"task_id": task.ID})
	}
}
