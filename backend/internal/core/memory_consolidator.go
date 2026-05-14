package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"memobase/backend/internal/store"
)

// MemoryConsolidator handles periodic memory maintenance:
// deduplication, long-term extraction, profile updates, and cleanup.
type MemoryConsolidator struct {
	app *App
}

func NewMemoryConsolidator(app *App) *MemoryConsolidator {
	return &MemoryConsolidator{app: app}
}

// Consolidate runs all consolidation steps for a single user.
func (mc *MemoryConsolidator) Consolidate(ctx context.Context, userID string) error {
	mc.app.Logger.Info("memory_consolidate_start", "user_id", userID)

	// Step 1: Deduplicate short-term memories
	if err := mc.deduplicate(ctx, userID); err != nil {
		mc.app.Logger.Warn("memory_dedup_failed", "user_id", userID, "error", err.Error())
	}

	// Step 2: Extract long-term memories from recent short-term
	if err := mc.extractLongTerm(ctx, userID); err != nil {
		mc.app.Logger.Warn("memory_extract_failed", "user_id", userID, "error", err.Error())
	}

	// Step 3: Cleanup expired and low-value memories
	if err := mc.cleanup(ctx, userID); err != nil {
		mc.app.Logger.Warn("memory_cleanup_failed", "user_id", userID, "error", err.Error())
	}

	mc.app.Logger.Info("memory_consolidate_done", "user_id", userID)
	return nil
}

// deduplicate removes similar short-term memories, keeping the one with higher importance.
func (mc *MemoryConsolidator) deduplicate(ctx context.Context, userID string) error {
	memories, err := mc.app.Store.ListUserMemories(ctx, userID, "short_term", 200)
	if err != nil {
		return err
	}

	// Group by normalized summary prefix (first 50 chars)
	type group struct {
		keep    store.Memory
		mergeIDs []string
	}
	groups := make(map[string]*group)

	for _, m := range memories {
		key := normalizeSummary(m.Summary)
		if existing, ok := groups[key]; ok {
			if m.Importance > existing.keep.Importance {
				existing.mergeIDs = append(existing.mergeIDs, existing.keep.ID)
				existing.keep = m
			} else {
				existing.mergeIDs = append(existing.mergeIDs, m.ID)
			}
		} else {
			groups[key] = &group{keep: m}
		}
	}

	for _, g := range groups {
		if len(g.mergeIDs) > 0 {
			if err := mc.app.Store.MergeMemories(ctx, g.keep.ID, g.mergeIDs); err != nil {
				mc.app.Logger.Warn("merge_failed", "keep_id", g.keep.ID, "error", err.Error())
			}
		}
	}

	return nil
}

// normalizeSummary returns a lowercase prefix for dedup comparison.
func normalizeSummary(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	runes := []rune(s)
	if len(runes) > 30 {
		return string(runes[:30])
	}
	return s
}

const extractLongTermPrompt = `分析以下对话记忆片段，提取值得长期保存的信息。

记忆片段：
%s

请提取以下类型的信息（JSON数组输出）：
- type: "long_term" — 用户的知识、经验、工作内容
- type: "fact" — 用户提到的具体事实（公司、技术栈、项目等）
- type: "preference" — 用户的偏好（回答风格、技术倾向等）

每个条目包含：
- type: 类型
- summary: 一句话摘要（30字内）
- importance: 0.0-1.0 重要性评分

仅输出JSON数组，无相关内容输出空数组 []`

type extractedMemory struct {
	Type       string  `json:"type"`
	Summary    string  `json:"summary"`
	Importance float64 `json:"importance"`
}

// extractLongTerm uses LLM to extract long-term memories from recent short-term ones.
func (mc *MemoryConsolidator) extractLongTerm(ctx context.Context, userID string) error {
	// Get recent short-term memories (last 20)
	shortTerms, err := mc.app.Store.ListUserMemories(ctx, userID, "short_term", 20)
	if err != nil || len(shortTerms) < 3 {
		return err // not enough to extract
	}

	// Check if we already extracted from these recently
	existingLongTerm, _ := mc.app.Store.ListUserMemories(ctx, userID, "long_term", 5)
	if len(existingLongTerm) > 0 {
		lastExtract := existingLongTerm[0].CreatedAt
		recentCount := 0
		for _, st := range shortTerms {
			if st.CreatedAt.After(lastExtract) {
				recentCount++
			}
		}
		if recentCount < 3 {
			return nil // not enough new memories since last extraction
		}
	}

	// Build prompt from short-term memories
	var memText strings.Builder
	for _, m := range shortTerms {
		fmt.Fprintf(&memText, "- [%s] %s\n", m.Type, m.Summary)
	}

	provider := mc.getProvider()
	if provider == nil {
		return fmt.Errorf("no provider available for extraction")
	}

	prompt := fmt.Sprintf(extractLongTermPrompt, memText.String())
	answer, _, _, err := mc.app.Provider.Chat(ctx, provider.baseURL, provider.apiKey, provider.model, prompt)
	if err != nil {
		return fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Parse JSON array from response
	var extracted []extractedMemory
	answer = strings.TrimSpace(answer)
	// Try to extract JSON array from response
	if idx := strings.Index(answer, "["); idx >= 0 {
		if endIdx := strings.LastIndex(answer, "]"); endIdx > idx {
			answer = answer[idx : endIdx+1]
		}
	}
	if err := json.Unmarshal([]byte(answer), &extracted); err != nil {
		return fmt.Errorf("failed to parse extraction result: %w", err)
	}

	// Create long-term memories
	for _, e := range extracted {
		if e.Summary == "" || e.Importance < 0.3 {
			continue
		}
		if e.Type != "long_term" && e.Type != "fact" && e.Type != "preference" {
			continue
		}
		_, err := mc.app.Store.CreateUserMemory(ctx, userID, e.Type, e.Summary, e.Importance)
		if err != nil {
			mc.app.Logger.Warn("create_extracted_memory_failed", "error", err.Error())
		}
	}

	return nil
}

type providerConfig struct {
	baseURL string
	apiKey  string
	model   string
}

func (mc *MemoryConsolidator) getProvider() *providerConfig {
	// Use the first default provider from DB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	providers, err := mc.app.Store.ListModelProviders(ctx, "")
	if err != nil || len(providers) == 0 {
		return nil
	}

	// Find default or use first
	for _, p := range providers {
		if p.IsDefault {
			return &providerConfig{baseURL: p.APIBaseURL, apiKey: p.APIKey, model: p.DefaultModel}
		}
	}
	p := providers[0]
	return &providerConfig{baseURL: p.APIBaseURL, apiKey: p.APIKey, model: p.DefaultModel}
}

// cleanup removes expired and low-value memories.
func (mc *MemoryConsolidator) cleanup(ctx context.Context, userID string) error {
	// Delete expired
	affected, err := mc.app.Store.DeleteExpiredMemories(ctx)
	if err != nil {
		return err
	}
	if affected > 0 {
		mc.app.Logger.Info("deleted_expired_memories", "count", affected)
	}

	// Delete low-importance old memories (importance < 0.2, access_count == 0, > 30 days)
	threshold := time.Now().AddDate(0, 0, -30)
	count, err := mc.app.Store.DeleteLowValueMemories(ctx, userID, 0.2, threshold)
	if err != nil {
		return err
	}
	if count > 0 {
		mc.app.Logger.Info("deleted_low_value_memories", "count", count)
	}

	// Enforce max limits
	if mc.app.Config.MemoryMaxLongTerm > 0 {
		deleted, err := mc.app.Store.TrimUserMemories(ctx, userID, "long_term", mc.app.Config.MemoryMaxLongTerm)
		if err != nil {
			return err
		}
		if deleted > 0 {
			mc.app.Logger.Info("trimmed_long_term_memories", "count", deleted)
		}
	}

	return nil
}

// StartMemoryConsolidator starts the periodic consolidation goroutine.
func (a *App) StartMemoryConsolidator() {
	interval := a.Config.MemoryConsolidateInterval
	if interval <= 0 {
		interval = 60 * time.Minute
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		mc := NewMemoryConsolidator(a)

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.consolidateAllUsers(mc)
			}
		}
	}()

	a.Logger.Info("memory_consolidator_started", "interval", interval.String())
}

func (a *App) consolidateAllUsers(mc *MemoryConsolidator) {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()

	userIDs, err := a.Store.ListAllUserIDs(ctx)
	if err != nil {
		a.Logger.Error("list_users_for_consolidation_failed", "error", err.Error())
		return
	}

	for _, uid := range userIDs {
		if err := mc.Consolidate(ctx, uid); err != nil {
			a.Logger.Warn("consolidation_user_failed", "user_id", uid, "error", err.Error())
		}
	}
}
