package api

import (
	"context"
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
			ProviderID   string `json:"provider_id"`
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
		topK := clampTopK(req.TopK)

		// Resolve provider config if provider_id is set
		var providerBaseURL, providerAPIKey, providerModel string
		if req.ProviderID != "" {
			userID := userIDFrom(c)
			mp, err := app.Store.GetModelProvider(c.Request.Context(), userID, req.ProviderID)
			if err != nil {
				if store.IsNotFound(err) {
					util.Fail(c, http.StatusNotFound, "PROVIDER_NOT_FOUND", "model provider not found", nil)
				} else {
					util.Internal(c, "failed to get provider")
				}
				return
			}
			providerBaseURL = mp.APIBaseURL
			providerAPIKey = mp.APIKey
			providerModel = mp.DefaultModel
			app.Logger.Info("using_external_provider",
				"provider_id", req.ProviderID,
				"provider_name", mp.Name,
				"model", providerModel,
				"api_base_url", providerBaseURL,
			)
		}

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

		if !useAgent || app.Agent == nil {
			util.Fail(c, http.StatusServiceUnavailable, "AGENT_UNAVAILABLE", "agent service is required for chat", nil)
			return
		}

		handleChatViaAgent(c, app, startedAt, sessionID, req.KBID, question, model, topK, req.ProviderID, providerBaseURL, providerAPIKey, providerModel)
	}
}

// ── Chat Stream (SSE) ────────────────────────────────────────────────────────

func handleChatStream(app *core.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			KBID       string `json:"kb_id"`
			SessionID  string `json:"session_id"`
			Question   string `json:"question"`
			Model      string `json:"model"`
			ProviderID string `json:"provider_id"`
			TopK       int    `json:"top_k"`
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

		// Resolve provider config if provider_id is set
		var providerBaseURL, providerAPIKey, providerModel string
		if req.ProviderID != "" {
			userID := userIDFrom(c)
			mp, err := app.Store.GetModelProvider(c.Request.Context(), userID, req.ProviderID)
			if err != nil {
				if store.IsNotFound(err) {
					util.Fail(c, http.StatusNotFound, "PROVIDER_NOT_FOUND", "model provider not found", nil)
				} else {
					util.Internal(c, "failed to get provider")
				}
				return
			}
			providerBaseURL = mp.APIBaseURL
			providerAPIKey = mp.APIKey
			providerModel = mp.DefaultModel
			app.Logger.Info("using_external_provider",
				"provider_id", req.ProviderID,
				"provider_name", mp.Name,
				"model", providerModel,
				"api_base_url", providerBaseURL,
			)
		}

		sessionID, sessionErr := ensureSession(c, app, req.KBID, req.SessionID, question)
		if sessionErr != nil {
			return
		}

		if _, err := app.Store.CreateMessage(c.Request.Context(), sessionID, "user", question); err != nil {
			util.Internal(c, "failed to write user message")
			return
		}

		if app.Agent == nil {
			util.Fail(c, http.StatusServiceUnavailable, "AGENT_UNAVAILABLE", "agent service is required for chat", nil)
			return
		}

		handleChatStreamViaAgent(c, app, sessionID, req.KBID, question, req.Model, req.TopK, req.ProviderID, providerBaseURL, providerAPIKey, providerModel)
	}
}

// ── Agent path ───────────────────────────────────────────────────────────────

func handleChatViaAgent(c *gin.Context, app *core.App, startedAt time.Time, sessionID, kbID, question, model string, topK int, providerID, providerBaseURL, providerAPIKey, providerModel string) {
	ctx := c.Request.Context()

	// Load chat history — fetch more, let budget manage
	messages, _, histErr := app.Store.ListMessages(ctx, sessionID, 200, 0)
	if histErr != nil {
		app.Logger.Warn("load_history_failed", "session_id", sessionID, "error", histErr.Error())
	}

	// Load memories (session-level + user-level)
	sessionMems, memErr := app.Store.ListSessionMemories(ctx, sessionID, 3)
	if memErr != nil {
		app.Logger.Warn("load_session_memories_failed", "session_id", sessionID, "error", memErr.Error())
	}
	var memories []store.Memory
	memories = append(memories, sessionMems...)

	// Load user-level memories (long_term, fact, preference)
	if userID, err := app.Store.GetKBUserID(ctx, kbID); err == nil {
		userMems, err := app.Store.ListUserMemories(ctx, userID, "", 5)
		if err != nil {
			app.Logger.Warn("load_user_memories_failed", "user_id", userID, "error", err.Error())
		} else {
			// Deduplicate by ID
			seen := make(map[string]bool)
			for _, m := range sessionMems {
				seen[m.ID] = true
			}
			for _, m := range userMems {
				if !seen[m.ID] {
					memories = append(memories, m)
				}
			}
		}
	}

	// Context budget management
	budget := core.NewContextBudget(app.Config.ContextWindow, app.Config.OutputReserve)
	budget.SystemTokens = 500
	budget.RAGTokens = 1500
	budget.MemoryTokens = core.EstimateMemoryTokens(memories)

	var llmCompressFn func([]store.Message) (string, error)
	if app.Provider != nil && providerBaseURL != "" && providerAPIKey != "" {
		llmCompressFn = func(msgs []store.Message) (string, error) {
			return llmCompressHistory(ctx, app, providerBaseURL, providerAPIKey, providerModel, msgs)
		}
	}

	managed := budget.Manage(messages, memories, llmCompressFn)

	if len(managed.ActionsTaken) > 0 {
		app.Logger.Info("context_managed",
			"session_id", sessionID,
			"pressure", managed.Pressure.String(),
			"actions", joinStrings(managed.ActionsTaken, ","),
			"tokens", managed.TokenCount,
		)
	}

	// Write back compressed memories from L3
	for _, m := range managed.Memories {
		if m.Type == "compressed" && m.ID == "" {
			if _, err := app.Store.CreateMemory(ctx, sessionID, "compressed", m.Summary); err != nil {
				app.Logger.Warn("save_compressed_memory_failed", "session_id", sessionID, "error", err.Error())
			}
		}
	}

	// Build gRPC messages from managed context
	history := make([]*pb.Message, 0, len(managed.Messages))
	for _, m := range managed.Messages {
		history = append(history, &pb.Message{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	memoryPbs := make([]*pb.Memory, 0, len(managed.Memories))
	for _, m := range managed.Memories {
		memoryPbs = append(memoryPbs, &pb.Memory{
			Type:    m.Type,
			Summary: m.Summary,
		})
	}

	grpcReq := &pb.ChatRequest{
		SessionId:          sessionID,
		KbId:               kbID,
		Question:           question,
		Model:              model,
		TopK:               int32(topK),
		History:            history,
		Memories:           memoryPbs,
		ProviderId:         providerID,
		ProviderApiBaseUrl: providerBaseURL,
		ProviderApiKey:     providerAPIKey,
		ProviderModel:      providerModel,
	}

	resp, err := app.Agent.ChatCompletion(ctx, grpcReq)
	if err != nil {
		app.Logger.Warn("agent_call_failed", "error", err.Error())
		util.Fail(c, http.StatusServiceUnavailable, "AGENT_ERROR", "agent chat failed: "+err.Error(), nil)
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
		"context_managed": gin.H{
			"pressure": managed.Pressure.String(),
			"actions":  managed.ActionsTaken,
			"tokens":   managed.TokenCount,
		},
		"memory_count": len(managed.Memories),
	})
}

// ── Streaming via agent ─────────────────────────────────────────────────────

func handleChatStreamViaAgent(c *gin.Context, app *core.App, sessionID, kbID, question, model string, topK int, providerID, providerBaseURL, providerAPIKey, providerModel string) {
	ctx := c.Request.Context()

	// Load history — fetch more, let budget manage
	messages, _, histErr := app.Store.ListMessages(ctx, sessionID, 200, 0)
	if histErr != nil {
		app.Logger.Warn("load_history_failed", "session_id", sessionID, "error", histErr.Error())
	}

	// Load memories (session-level + user-level)
	sessionMems, memErr := app.Store.ListSessionMemories(ctx, sessionID, 3)
	if memErr != nil {
		app.Logger.Warn("load_session_memories_failed", "session_id", sessionID, "error", memErr.Error())
	}
	var memories []store.Memory
	memories = append(memories, sessionMems...)

	if userID, err := app.Store.GetKBUserID(ctx, kbID); err == nil {
		userMems, err := app.Store.ListUserMemories(ctx, userID, "", 5)
		if err != nil {
			app.Logger.Warn("load_user_memories_failed", "user_id", userID, "error", err.Error())
		} else {
			seen := make(map[string]bool)
			for _, m := range sessionMems {
				seen[m.ID] = true
			}
			for _, m := range userMems {
				if !seen[m.ID] {
					memories = append(memories, m)
				}
			}
		}
	}

	// Context budget management
	budget := core.NewContextBudget(app.Config.ContextWindow, app.Config.OutputReserve)
	budget.SystemTokens = 500
	budget.RAGTokens = 1500
	budget.MemoryTokens = core.EstimateMemoryTokens(memories)

	var llmCompressFn func([]store.Message) (string, error)
	if app.Provider != nil && providerBaseURL != "" && providerAPIKey != "" {
		llmCompressFn = func(msgs []store.Message) (string, error) {
			return llmCompressHistory(ctx, app, providerBaseURL, providerAPIKey, providerModel, msgs)
		}
	}

	managed := budget.Manage(messages, memories, llmCompressFn)

	if len(managed.ActionsTaken) > 0 {
		app.Logger.Info("context_managed",
			"session_id", sessionID,
			"pressure", managed.Pressure.String(),
			"actions", joinStrings(managed.ActionsTaken, ","),
			"tokens", managed.TokenCount,
		)
	}

	// Write back compressed memories from L3
	for _, m := range managed.Memories {
		if m.Type == "compressed" && m.ID == "" {
			if _, err := app.Store.CreateMemory(ctx, sessionID, "compressed", m.Summary); err != nil {
				app.Logger.Warn("save_compressed_memory_failed", "session_id", sessionID, "error", err.Error())
			}
		}
	}

	// Build gRPC messages from managed context
	history := make([]*pb.Message, 0, len(managed.Messages))
	for _, m := range managed.Messages {
		history = append(history, &pb.Message{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	memoryPbs := make([]*pb.Memory, 0, len(managed.Memories))
	for _, m := range managed.Memories {
		memoryPbs = append(memoryPbs, &pb.Memory{
			Type:    m.Type,
			Summary: m.Summary,
		})
	}

	grpcReq := &pb.ChatRequest{
		SessionId:          sessionID,
		KbId:               kbID,
		Question:           question,
		Model:              model,
		TopK:               int32(topK),
		History:            history,
		Memories:           memoryPbs,
		ProviderId:         providerID,
		ProviderApiBaseUrl: providerBaseURL,
		ProviderApiKey:     providerAPIKey,
		ProviderModel:      providerModel,
	}

	stream, err := app.Agent.ChatCompletionStream(ctx, grpcReq)
	if err != nil {
		app.Logger.Warn("agent_stream_failed", "error", err.Error())
		util.Fail(c, http.StatusServiceUnavailable, "AGENT_ERROR", "agent stream failed: "+err.Error(), nil)
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
				"type":         "result",
				"answer":       ev.Result.Answer,
				"degraded":     ev.Result.Degraded,
				"latency_ms":   ev.Result.LatencyMs,
				"memory_count": len(managed.Memories),
				"context_managed": map[string]interface{}{
					"pressure": managed.Pressure.String(),
					"actions":  managed.ActionsTaken,
					"tokens":   managed.TokenCount,
				},
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

const compressPrompt = `请将以下对话压缩为一段结构化摘要。

保留:
1. 关键事实和数据
2. 用户的核心意图和需求
3. 未完成的任务或待确认事项
4. 重要结论和决定
5. 技术细节中的关键参数/配置

丢弃:
1. 寒暄和过程性对话
2. 重复内容
3. 工具调用的中间过程
4. LLM的推理过程

对话内容:
%s

输出格式:
## 对话摘要
{200字以内的核心摘要}

## 关键事实
- {事实1}
- {事实2}

## 未完成事项
- {待办1}

请直接输出，不要添加其他说明。`

func llmCompressHistory(ctx context.Context, app *core.App, apiBaseURL, apiKey, model string, messages []store.Message) (string, error) {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
	}

	prompt := fmt.Sprintf(compressPrompt, b.String())

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	answer, _, _, err := app.Provider.Chat(ctx, apiBaseURL, apiKey, model, prompt)
	if err != nil {
		return "", err
	}
	return answer, nil
}

func joinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

