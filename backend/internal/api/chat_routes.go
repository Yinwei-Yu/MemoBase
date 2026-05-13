package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"memobase/backend/internal/core"
	"memobase/backend/internal/infra"
	"memobase/backend/internal/store"
	"memobase/backend/internal/util"

	pb "memobase/backend/proto"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&chatRegistrar{})
}

type chatRegistrar struct{}

func (chatRegistrar) Register(_ *gin.RouterGroup, authed *gin.RouterGroup, app *core.App) {
	authed.POST("/chat/completions", handleChatCompletions(app))
	authed.POST("/chat/completions/stream", handleChatStream(app))
	authed.GET("/chat/traces/:trace_id", handleGetTrace(app))
}

// ── Chat Completions (unary) ─────────────────────────────────────────────────

func handleChatCompletions(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		var req struct {
			KBID         string `json:"kb_id"`
			SessionID    string `json:"session_id"`
			Question     string `json:"question"`
			Model        string `json:"model"`
			UseAgent     *bool  `json:"use_agent"`
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

		sessionID, sessionErr := ensureSession(c, app, req.KBID, req.SessionID, question)
		if sessionErr != nil {
			return
		}

		if _, err := app.Store.CreateMessage(c.Request.Context(), sessionID, "user", question); err != nil {
			util.Internal(c, "failed to write user message")
			return
		}

		// Determine whether to use agent
		useAgent := app.Agent != nil
		if req.UseAgent != nil {
			useAgent = *req.UseAgent && app.Agent != nil
		}

		if useAgent {
			handleChatViaAgent(c, app, startedAt, sessionID, req.KBID, question, model, topK)
			return
		}

		// Fallback: local RAG path
		handleChatLocal(c, app, startedAt, sessionID, req.KBID, question, model, topK)
	}
}

// ── Chat Stream (SSE) ────────────────────────────────────────────────────────

func handleChatStream(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			KBID      string `json:"kb_id"`
			SessionID string `json:"session_id"`
			Question  string `json:"question"`
			Model     string `json:"model"`
			TopK      int    `json:"top_k"`
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

		sessionID, sessionErr := ensureSession(c, app, req.KBID, req.SessionID, question)
		if sessionErr != nil {
			return
		}

		if _, err := app.Store.CreateMessage(c.Request.Context(), sessionID, "user", question); err != nil {
			util.Internal(c, "failed to write user message")
			return
		}

		// If agent is not available, fall back to unary mode via SSE
		if app.Agent == nil {
			handleChatStreamLocal(c, app, sessionID, req.KBID, question, req.Model, req.TopK)
			return
		}

		handleChatStreamViaAgent(c, app, sessionID, req.KBID, question, req.Model, req.TopK)
	}
}

// ── Agent path ───────────────────────────────────────────────────────────────

func handleChatViaAgent(c *gin.Context, app *core.App, startedAt time.Time, sessionID, kbID, question, model string, topK int) {
	ctx := c.Request.Context()

	// Load chat history
	messages, _, histErr := app.Store.ListMessages(ctx, sessionID, 20, 0)
	if histErr != nil {
		app.Logger.Warn("load_history_failed", "session_id", sessionID, "error", histErr.Error())
	}
	history := make([]*pb.Message, 0, len(messages))
	for _, m := range messages {
		history = append(history, &pb.Message{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	// Load memories
	memories, memErr := app.Store.ListSessionMemories(ctx, sessionID, 5)
	if memErr != nil {
		app.Logger.Warn("load_memories_failed", "session_id", sessionID, "error", memErr.Error())
	}
	memoryPbs := make([]*pb.Memory, 0, len(memories))
	for _, m := range memories {
		memoryPbs = append(memoryPbs, &pb.Memory{
			Type:    m.Type,
			Summary: m.Summary,
		})
	}

	grpcReq := &pb.ChatRequest{
		SessionId: sessionID,
		KbId:      kbID,
		Question:  question,
		Model:     model,
		TopK:      int32(topK),
		History:   history,
		Memories:  memoryPbs,
	}

	resp, err := app.Agent.ChatCompletion(ctx, grpcReq)
	if err != nil {
		app.Logger.Warn("agent_call_failed", "error", err.Error())
		// Fallback to local
		handleChatLocal(c, app, startedAt, sessionID, kbID, question, model, topK)
		return
	}

	// Save assistant message
	if _, err := app.Store.CreateMessage(ctx, sessionID, "assistant", resp.Answer); err != nil {
		util.Internal(c, "failed to write assistant message")
		return
	}
	_, _ = app.Store.CreateMemory(ctx, sessionID, "short_term",
		"Q: "+coreSummary(question, 80)+" | A: "+coreSummary(resp.Answer, 120))

	// Build citations
	citations := make([]core.Citation, 0, len(resp.Citations))
	for _, ct := range resp.Citations {
		citations = append(citations, core.Citation{
			DocID:           ct.DocId,
			DocTitle:        ct.DocTitle,
			ChunkID:         ct.ChunkId,
			Snippet:         ct.Snippet,
			Score:           ct.Score,
			RetrievalSource: ct.RetrievalSource,
		})
	}

	// Save trace if available
	traceID := ""
	if len(resp.Trace) > 0 {
		steps := make([]map[string]interface{}, 0, len(resp.Trace))
		for _, t := range resp.Trace {
			data := make(map[string]interface{})
			for k, v := range t.Data {
				data[k] = v
			}
			steps = append(steps, map[string]interface{}{
				"node":        t.Node,
				"action":      t.Action,
				"duration_ms": t.DurationMs,
				"data":        data,
			})
		}
		trace, err := app.Store.CreateTrace(ctx, sessionID, steps)
		if err == nil {
			traceID = trace.ID
		}
	}

	util.Success(c, http.StatusOK, gin.H{
		"session_id": sessionID,
		"answer":     resp.Answer,
		"citations":  citations,
		"trace_id":   traceID,
		"degraded":   resp.Degraded,
		"latency_ms": time.Since(startedAt).Milliseconds(),
		"token_usage": core.TokenUsage{
			PromptTokens:     int(resp.TokenUsage.PromptTokens),
			CompletionTokens: int(resp.TokenUsage.CompletionTokens),
			TotalTokens:      int(resp.TokenUsage.TotalTokens),
		},
	})
}

// ── Local fallback path ─────────────────────────────────────────────────────

func handleChatLocal(c *gin.Context, app *core.App, startedAt time.Time, sessionID, kbID, question, model string, topK int) {
	ctx := c.Request.Context()

	chunks, retrievalDegraded, err := app.RetrieveChunks(ctx, kbID, question, topK)
	if err != nil {
		app.Logger.Error("retrieval_failed",
			"request_id", util.RequestID(c),
			"kb_id", kbID,
			"error", err.Error(),
		)
		util.Fail(c, http.StatusServiceUnavailable, "QDRANT_UNAVAILABLE", "retrieval failed", nil)
		return
	}
	degraded := retrievalDegraded
	memories, _ := app.Store.ListSessionMemories(ctx, sessionID, 5)
	prompt := app.BuildChatPrompt(question, chunks, memories)
	answer, promptT, completionT, err := app.Ollama.Chat(ctx, model, prompt)
	if err != nil {
		app.Logger.Error("model_chat_failed",
			"request_id", util.RequestID(c),
			"session_id", sessionID,
			"error", err.Error(),
		)
		util.Fail(c, http.StatusServiceUnavailable, "MODEL_UNAVAILABLE", "ollama chat failed", nil)
		return
	}

	if _, err := app.Store.CreateMessage(ctx, sessionID, "assistant", answer); err != nil {
		util.Internal(c, "failed to write assistant message")
		return
	}
	_, _ = app.Store.CreateMemory(ctx, sessionID, "short_term",
		"Q: "+coreSummary(question, 80)+" | A: "+coreSummary(answer, 120))

	citations := make([]core.Citation, 0, len(chunks))
	for _, ch := range chunks {
		doc, _ := app.Store.GetDocument(ctx, kbID, ch.DocID)
		citations = append(citations, core.Citation{
			DocID:           ch.DocID,
			DocTitle:        doc.FileName,
			ChunkID:         ch.ChunkID,
			Snippet:         coreSummary(ch.Content, 160),
			Score:           ch.Score,
			RetrievalSource: ch.Src,
		})
	}

	util.Success(c, http.StatusOK, gin.H{
		"session_id":  sessionID,
		"answer":      answer,
		"citations":   citations,
		"memory_used": memories,
		"degraded":    degraded,
		"latency_ms":  time.Since(startedAt).Milliseconds(),
		"token_usage": core.TokenUsage{PromptTokens: promptT, CompletionTokens: completionT, TotalTokens: promptT + completionT},
	})
}

// ── Streaming via agent ─────────────────────────────────────────────────────

func handleChatStreamViaAgent(c *gin.Context, app *core.App, sessionID, kbID, question, model string, topK int) {
	ctx := c.Request.Context()

	// Load history
	messages, _, histErr := app.Store.ListMessages(ctx, sessionID, 20, 0)
	if histErr != nil {
		app.Logger.Warn("load_history_failed", "session_id", sessionID, "error", histErr.Error())
	}
	history := make([]*pb.Message, 0, len(messages))
	for _, m := range messages {
		history = append(history, &pb.Message{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}

	memories, memErr := app.Store.ListSessionMemories(ctx, sessionID, 5)
	if memErr != nil {
		app.Logger.Warn("load_memories_failed", "session_id", sessionID, "error", memErr.Error())
	}
	memoryPbs := make([]*pb.Memory, 0, len(memories))
	for _, m := range memories {
		memoryPbs = append(memoryPbs, &pb.Memory{
			Type:    m.Type,
			Summary: m.Summary,
		})
	}

	grpcReq := &pb.ChatRequest{
		SessionId: sessionID,
		KbId:      kbID,
		Question:  question,
		Model:     model,
		TopK:      int32(topK),
		History:   history,
		Memories:  memoryPbs,
	}

	stream, err := app.Agent.ChatCompletionStream(ctx, grpcReq)
	if err != nil {
		app.Logger.Warn("agent_stream_failed", "error", err.Error())
		handleChatStreamLocal(c, app, sessionID, kbID, question, model, topK)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		util.Internal(c, "streaming not supported")
		return
	}

	var fullAnswer string
	for event := range infra.RecvEvents(stream) {
		if event.Err == io.EOF {
			break
		}
		if event.Err != nil {
			app.Logger.Warn("stream_event_error", "error", event.Err.Error())
			// Send error event
			data, _ := json.Marshal(map[string]interface{}{
				"type":    "error",
				"message": event.Err.Error(),
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()
			break
		}
		if event.Event == nil {
			continue
		}

		switch ev := event.Event.Event.(type) {
		case *pb.ChatEvent_Step:
			data, _ := json.Marshal(map[string]interface{}{
				"type":   "step",
				"node":   ev.Step.StepName,
				"status": ev.Step.Status,
				"detail": ev.Step.Detail,
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()

		case *pb.ChatEvent_Token:
			fullAnswer += ev.Token.Token
			data, _ := json.Marshal(map[string]interface{}{
				"type":  "token",
				"token": ev.Token.Token,
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()

		case *pb.ChatEvent_Result:
			// Save assistant message
			_, _ = app.Store.CreateMessage(ctx, sessionID, "assistant", fullAnswer)
			_, _ = app.Store.CreateMemory(ctx, sessionID, "short_term",
				"Q: "+coreSummary(question, 80)+" | A: "+coreSummary(fullAnswer, 120))

			data, _ := json.Marshal(map[string]interface{}{
				"type":       "result",
				"answer":     ev.Result.Answer,
				"degraded":   ev.Result.Degraded,
				"latency_ms": ev.Result.LatencyMs,
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()

		case *pb.ChatEvent_Error:
			data, _ := json.Marshal(map[string]interface{}{
				"type":    "error",
				"code":    ev.Error.Code,
				"message": ev.Error.Message,
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// ── Streaming local fallback (SSE wrapping unary) ────────────────────────────

func handleChatStreamLocal(c *gin.Context, app *core.App, sessionID, kbID, question, model string, topK int) {
	ctx := c.Request.Context()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		util.Internal(c, "streaming not supported")
		return
	}

	// Send start event
	data, _ := json.Marshal(map[string]interface{}{
		"type":   "step",
		"node":   "retrieve",
		"status": "started",
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()

	chunks, _, err := app.RetrieveChunks(ctx, kbID, question, topK)
	if err != nil {
		data, _ := json.Marshal(map[string]interface{}{"type": "error", "message": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	data, _ = json.Marshal(map[string]interface{}{
		"type":   "step",
		"node":   "retrieve",
		"status": "completed",
		"detail": fmt.Sprintf("found %d chunks", len(chunks)),
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()

	memories, _ := app.Store.ListSessionMemories(ctx, sessionID, 5)
	prompt := app.BuildChatPrompt(question, chunks, memories)
	answer, _, _, err := app.Ollama.Chat(ctx, model, prompt)
	if err != nil {
		data, _ := json.Marshal(map[string]interface{}{"type": "error", "message": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Stream tokens (simple character-by-character for local fallback)
	for i, r := range answer {
		data, _ := json.Marshal(map[string]interface{}{
			"type":  "token",
			"token": string(r),
			"index": i,
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	_, _ = app.Store.CreateMessage(ctx, sessionID, "assistant", answer)
	_, _ = app.Store.CreateMemory(ctx, sessionID, "short_term",
		"Q: "+coreSummary(question, 80)+" | A: "+coreSummary(answer, 120))

	data, _ = json.Marshal(map[string]interface{}{
		"type":     "result",
		"answer":   answer,
		"degraded": false,
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	flusher.Flush()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func ensureSession(c *gin.Context, app *core.App, kbID, sessionID, question string) (string, error) {
	userID := userIDFrom(c)
	if strings.TrimSpace(sessionID) == "" {
		// Verify KB belongs to user before creating session
		if _, err := app.Store.GetKB(c.Request.Context(), userID, kbID); err != nil {
			if store.IsNotFound(err) {
				util.Fail(c, http.StatusNotFound, "KB_NOT_FOUND", "knowledge base not found", nil)
			} else {
				util.Internal(c, "failed to get knowledge base")
			}
			return "", err
		}
		s, err := app.Store.CreateSession(c.Request.Context(), userID, kbID, "会话: "+coreSummary(question, 12))
		if err != nil {
			util.Internal(c, "failed to create session")
			return "", err
		}
		return s.ID, nil
	}
	s, err := app.Store.GetSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		if store.IsNotFound(err) {
			util.Fail(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found", nil)
		} else {
			util.Internal(c, "failed to get session")
		}
		return "", err
	}
	if s.KBID != kbID {
		util.Fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "session_id does not belong to kb_id", nil)
		return "", fmt.Errorf("session/kb mismatch")
	}
	return sessionID, nil
}

func handleGetTrace(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
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
	}
}
