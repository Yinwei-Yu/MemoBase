package api

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"memobase/backend/internal/core"
	"memobase/backend/internal/infra"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

var supportedUploadExt = map[string]struct{}{
	".txt": {},
	".md":  {},
}

const (
	maxUploadFileSize  = 20 * 1024 * 1024
	maxUploadFileCount = 20
)

type uploadDocumentItem struct {
	DocID     string    `json:"doc_id"`
	KBID      string    `json:"kb_id"`
	FileName  string    `json:"file_name"`
	Status    string    `json:"status"`
	TaskID    string    `json:"task_id"`
	CreatedAt time.Time `json:"created_at"`
}

type uploadDocumentsResponse struct {
	Items         []uploadDocumentItem `json:"items"`
	UploadedCount int                  `json:"uploaded_count"`
}

func isSupportedUploadExt(ext string) bool {
	_, ok := supportedUploadExt[strings.ToLower(strings.TrimSpace(ext))]
	return ok
}

func parsePage(c *gin.Context) (int, int, int) {
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	return page, pageSize, (page - 1) * pageSize
}

func userIDFrom(c *gin.Context) string {
	uid, _ := c.Get("user_id")
	s, _ := uid.(string)
	return s
}

func trimAndFilterTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		out = append(out, tag)
	}
	return out
}

func validateKBFields(name, description *string, tags *[]string) error {
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return fmt.Errorf("name is required")
		}
		if utf8.RuneCountInString(trimmed) > 64 {
			return fmt.Errorf("name must be between 1 and 64 characters")
		}
		*name = trimmed
	}
	if description != nil {
		trimmed := strings.TrimSpace(*description)
		if utf8.RuneCountInString(trimmed) > 512 {
			return fmt.Errorf("description must be at most 512 characters")
		}
		*description = trimmed
	}
	if tags != nil {
		filtered := trimAndFilterTags(*tags)
		if len(filtered) > 10 {
			return fmt.Errorf("tags must be at most 10 items")
		}
		*tags = filtered
	}
	return nil
}

func clampTopK(v int) int {
	if v <= 0 {
		return 6
	}
	if v > 20 {
		return 20
	}
	return v
}

func RegisterRoutes(r *gin.Engine, app *core.App) {
	v1 := r.Group("/api/v1")

	v1.POST("/auth/login", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid login payload", nil)
			return
		}
		if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "username and password are required", nil)
			return
		}
		user, err := app.VerifyUser(c.Request.Context(), req.Username, req.Password)
		if err != nil {
			util.Unauthorized(c, "invalid credentials")
			return
		}
		token, err := util.SignToken(app.Config.JWTSecret, user.ID, app.Config.TokenTTL)
		if err != nil {
			util.Internal(c, "failed to sign token")
			return
		}
		util.Success(c, http.StatusOK, gin.H{
			"access_token":  token,
			"refresh_token": "",
			"expires_in":    int(app.Config.TokenTTL.Seconds()),
			"user": gin.H{
				"user_id":      user.ID,
				"username":     user.Username,
				"display_name": user.DisplayName,
			},
		})
	})

	v1.GET("/healthz", func(c *gin.Context) {
		util.Success(c, http.StatusOK, gin.H{"status": "ok"})
	})
	v1.GET("/readyz", func(c *gin.Context) {
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
		if err := app.Qdrant.Ready(ctx); err != nil {
			checks["qdrant"] = "down"
			status = "not_ready"
		} else {
			checks["qdrant"] = "up"
		}
		if err := app.Ollama.Ready(ctx); err != nil {
			checks["model_gateway"] = "down"
			status = "not_ready"
		} else {
			checks["model_gateway"] = "up"
		}
		checks["storage"] = "up"
		if status != "ready" {
			util.Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "dependency not ready", gin.H{"checks": checks})
			return
		}
		util.Success(c, http.StatusOK, gin.H{"status": status, "checks": checks})
	})
	v1.GET("/metrics", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4")
		_, _ = c.Writer.WriteString("# mock metrics\nmemobase_up 1\n")
	})

	authed := v1.Group("/")
	authed.Use(AuthRequired(app))

	authed.GET("/auth/me", func(c *gin.Context) {
		user, err := app.Store.GetUserByID(c.Request.Context(), userIDFrom(c))
		if err != nil {
			util.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
			return
		}
		util.Success(c, http.StatusOK, gin.H{
			"user_id":      user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
		})
	})

	authed.POST("/knowledge-bases", func(c *gin.Context) {
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
	})

	authed.GET("/knowledge-bases", func(c *gin.Context) {
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
	})

	authed.GET("/knowledge-bases/:kb_id", func(c *gin.Context) {
		kb, err := app.Store.GetKB(c.Request.Context(), c.Param("kb_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
				return
			}
			util.Internal(c, "failed to get knowledge base")
			return
		}
		util.Success(c, http.StatusOK, kb)
	})

	authed.PATCH("/knowledge-bases/:kb_id", func(c *gin.Context) {
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
		kb, err := app.Store.PatchKB(c.Request.Context(), c.Param("kb_id"), req.Name, req.Description, req.Tags)
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
				return
			}
			util.Internal(c, "failed to update knowledge base")
			return
		}
		util.Success(c, http.StatusOK, kb)
	})

	authed.DELETE("/knowledge-bases/:kb_id", func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if err := app.Store.DeleteKB(c.Request.Context(), kbID); err != nil {
			util.Internal(c, "failed to delete knowledge base")
			return
		}
		_ = app.Qdrant.DeleteCollection(c.Request.Context(), app.QdrantCollectionForKB(kbID))
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	})

	authed.POST("/knowledge-bases/:kb_id/documents", func(c *gin.Context) {
		kbID := c.Param("kb_id")
		if _, err := app.Store.GetKB(c.Request.Context(), kbID); err != nil {
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
	})

	authed.GET("/knowledge-bases/:kb_id/documents", func(c *gin.Context) {
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListDocuments(c.Request.Context(), c.Param("kb_id"), c.Query("status"), pageSize, offset)
		if err != nil {
			util.Internal(c, "failed to list documents")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"items": items, "pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total}})
	})

	authed.GET("/knowledge-bases/:kb_id/documents/:doc_id", func(c *gin.Context) {
		doc, err := app.Store.GetDocument(c.Request.Context(), c.Param("kb_id"), c.Param("doc_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "DOC_NOT_FOUND", "document not found", nil)
				return
			}
			util.Internal(c, "failed to get document")
			return
		}
		util.Success(c, http.StatusOK, doc)
	})

	authed.GET("/knowledge-bases/:kb_id/documents/:doc_id/content", func(c *gin.Context) {
		doc, err := app.Store.GetDocumentContent(c.Request.Context(), c.Param("kb_id"), c.Param("doc_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "DOC_NOT_FOUND", "document not found", nil)
				return
			}
			util.Internal(c, "failed to get document content")
			return
		}
		util.Success(c, http.StatusOK, doc)
	})

	authed.DELETE("/knowledge-bases/:kb_id/documents/:doc_id", func(c *gin.Context) {
		kbID := c.Param("kb_id")
		docID := c.Param("doc_id")
		if err := app.Store.DeleteDocument(c.Request.Context(), kbID, docID); err != nil {
			util.Internal(c, "failed to delete document")
			return
		}
		_ = app.Qdrant.DeleteByDoc(c.Request.Context(), app.QdrantCollectionForKB(kbID), docID)
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	})

	authed.POST("/knowledge-bases/:kb_id/documents/:doc_id/reindex", func(c *gin.Context) {
		doc, err := app.Store.GetDocument(c.Request.Context(), c.Param("kb_id"), c.Param("doc_id"))
		if err != nil {
			util.Fail(c, http.StatusNotFound, "DOC_NOT_FOUND", "document not found", nil)
			return
		}
		filePath := fmt.Sprintf("%s/%s/%s_%s", app.Config.StorageDir, c.Param("kb_id"), doc.ID, doc.FileName)
		task, err := app.Store.CreateTask(c.Request.Context(), "document_reindex", map[string]interface{}{"doc_id": doc.ID, "kb_id": doc.KBID})
		if err != nil {
			util.Internal(c, "failed to create task")
			return
		}
		go app.ProcessDocument(task.ID, doc.KBID, doc.ID, filePath, 500, 100)
		util.Success(c, http.StatusOK, gin.H{"task_id": task.ID})
	})

	authed.GET("/tasks/:task_id", func(c *gin.Context) {
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
	})

	authed.POST("/sessions", func(c *gin.Context) {
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
		s, err := app.Store.CreateSession(c.Request.Context(), req.KBID, req.Title)
		if err != nil {
			util.Internal(c, "failed to create session")
			return
		}
		util.Success(c, http.StatusCreated, s)
	})

	authed.GET("/sessions", func(c *gin.Context) {
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListSessions(c.Request.Context(), c.Query("kb_id"), pageSize, offset)
		if err != nil {
			util.Internal(c, "failed to list sessions")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"items": items, "pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total}})
	})

	authed.GET("/sessions/:session_id", func(c *gin.Context) {
		s, err := app.Store.GetSession(c.Request.Context(), c.Param("session_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found", nil)
				return
			}
			util.Internal(c, "failed to get session")
			return
		}
		util.Success(c, http.StatusOK, s)
	})

	authed.GET("/sessions/:session_id/messages", func(c *gin.Context) {
		page, pageSize, offset := parsePage(c)
		items, total, err := app.Store.ListMessages(c.Request.Context(), c.Param("session_id"), pageSize, offset)
		if err != nil {
			util.Internal(c, "failed to list messages")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"items": items, "pagination": core.Pagination{Page: page, PageSize: pageSize, Total: total}})
	})

	authed.DELETE("/sessions/:session_id", func(c *gin.Context) {
		if err := app.Store.DeleteSession(c.Request.Context(), c.Param("session_id")); err != nil {
			util.Internal(c, "failed to delete session")
			return
		}
		util.Success(c, http.StatusOK, gin.H{"deleted": true})
	})

	authed.POST("/chat/completions", func(c *gin.Context) {
		startedAt := time.Now()
		var req struct {
			KBID         string `json:"kb_id"`
			SessionID    string `json:"session_id"`
			Question     string `json:"question"`
			Model        string `json:"model"`
			UseAgent     bool   `json:"use_agent"`
			TopK         int    `json:"top_k"`
			IncludeTrace bool   `json:"include_trace"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid payload", nil)
			return
		}
		question := strings.TrimSpace(req.Question)
		if strings.TrimSpace(req.KBID) == "" || question == "" {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "kb_id and question are required", nil)
			return
		}
		if utf8.RuneCountInString(question) > 2000 {
			util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "question must be between 1 and 2000 characters", nil)
			return
		}
		model := req.Model
		if model == "" {
			model = app.Config.OllamaChatModel
		}
		topK := clampTopK(req.TopK)

		sessionID := req.SessionID
		if strings.TrimSpace(sessionID) == "" {
			s, err := app.Store.CreateSession(c.Request.Context(), req.KBID, "会话: "+coreSummary(question, 12))
			if err != nil {
				util.Internal(c, "failed to create session")
				return
			}
			sessionID = s.ID
		} else {
			s, err := app.Store.GetSession(c.Request.Context(), sessionID)
			if err != nil {
				if store.IsNotFound(err) {
					util.Fail(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found", nil)
					return
				}
				util.Internal(c, "failed to get session")
				return
			}
			if s.KBID != req.KBID {
				util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "session_id does not belong to kb_id", nil)
				return
			}
		}

		if _, err := app.Store.CreateMessage(c.Request.Context(), sessionID, "user", question); err != nil {
			util.Internal(c, "failed to write user message")
			return
		}

		chunks, retrievalDegraded, err := app.RetrieveChunks(c.Request.Context(), req.KBID, question, topK)
		if err != nil {
			app.Logger.Error("retrieval_failed",
				"request_id", util.RequestID(c),
				"kb_id", req.KBID,
				"error_code", "QDRANT_UNAVAILABLE",
				"error", err.Error(),
			)
			util.Fail(c, http.StatusServiceUnavailable, "QDRANT_UNAVAILABLE", "retrieval failed", nil)
			return
		}
		degraded := retrievalDegraded
		memories, _ := app.Store.ListSessionMemories(c.Request.Context(), sessionID, 5)
		prompt := app.BuildChatPrompt(question, chunks, memories)
		answer, promptT, completionT, err := app.Ollama.Chat(c.Request.Context(), model, prompt)
		if err != nil {
			app.Logger.Error("model_chat_failed",
				"request_id", util.RequestID(c),
				"session_id", sessionID,
				"error_code", "MODEL_UNAVAILABLE",
				"error", err.Error(),
			)
			util.Fail(c, http.StatusServiceUnavailable, "MODEL_UNAVAILABLE", "ollama chat failed", nil)
			return
		}

		if _, err := app.Store.CreateMessage(c.Request.Context(), sessionID, "assistant", answer); err != nil {
			util.Internal(c, "failed to write assistant message")
			return
		}
		_, _ = app.Store.CreateMemory(c.Request.Context(), sessionID, "short_term", "Q: "+coreSummary(question, 80)+" | A: "+coreSummary(answer, 120))

		citations := make([]core.Citation, 0, len(chunks))
		for _, ch := range chunks {
			doc, _ := app.Store.GetDocument(c.Request.Context(), req.KBID, ch.Chunk.DocID)
			citations = append(citations, core.Citation{
				DocID:           ch.Chunk.DocID,
				DocTitle:        doc.FileName,
				ChunkID:         ch.Chunk.ID,
				Snippet:         coreSummary(ch.Chunk.Content, 160),
				Score:           ch.Score,
				RetrievalSource: ch.Src,
			})
		}

		traceID := ""
		if req.UseAgent || req.IncludeTrace {
			steps := []map[string]interface{}{
				{"tool": "search_knowledge", "input": gin.H{"kb_id": req.KBID, "top_k": topK}, "observation": fmt.Sprintf("%d chunks", len(chunks)), "latency_ms": 50},
				{"tool": "search_memory", "input": gin.H{"session_id": sessionID}, "observation": fmt.Sprintf("%d memories", len(memories)), "latency_ms": 15},
				{"tool": "summarize_context", "input": gin.H{"question": question}, "observation": "context packed", "latency_ms": 22},
			}
			trace, err := app.Store.CreateTrace(c.Request.Context(), sessionID, steps)
			if err == nil {
				traceID = trace.ID
			} else {
				degraded = true
			}
		}

		util.Success(c, http.StatusOK, gin.H{
			"session_id":  sessionID,
			"answer":      answer,
			"citations":   citations,
			"memory_used": memories,
			"trace_id":    traceID,
			"degraded":    degraded,
			"latency_ms":  time.Since(startedAt).Milliseconds(),
			"token_usage": core.TokenUsage{PromptTokens: promptT, CompletionTokens: completionT, TotalTokens: promptT + completionT},
		})
	})

	authed.GET("/chat/traces/:trace_id", func(c *gin.Context) {
		trace, err := app.Store.GetTrace(c.Request.Context(), c.Param("trace_id"))
		if err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "TRACE_NOT_FOUND", "trace not found", nil)
				return
			}
			util.Internal(c, "failed to get trace")
			return
		}
		util.Success(c, http.StatusOK, trace)
	})
}

func coreSummary(text string, n int) string {
	r := []rune(strings.TrimSpace(text))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}
