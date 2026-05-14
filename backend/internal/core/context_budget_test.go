package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"memobase/backend/internal/store"
)

func TestContextPressure_String(t *testing.T) {
	tests := []struct {
		p    ContextPressure
		want string
	}{
		{PressureNormal, "normal"},
		{PressureWarn, "warn"},
		{PressureCritical, "critical"},
		{PressureEmergency, "emergency"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("ContextPressure(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestNewContextBudget(t *testing.T) {
	b := NewContextBudget(4096, 1024)
	if b.Window != 4096 {
		t.Errorf("Window = %d, want 4096", b.Window)
	}
	if b.OutputReserve != 1024 {
		t.Errorf("OutputReserve = %d, want 1024", b.OutputReserve)
	}
}

func TestHistoryBudget(t *testing.T) {
	b := &ContextBudget{Window: 4096, OutputReserve: 1024}
	b.SystemTokens = 500
	b.RAGTokens = 1500
	b.MemoryTokens = 500
	// 4096 - 1024 - 500 - 1500 - 500 = 572
	got := b.HistoryBudget()
	if got != 572 {
		t.Errorf("HistoryBudget() = %d, want 572", got)
	}
}

func TestHistoryBudget_Minimum(t *testing.T) {
	b := &ContextBudget{Window: 100, OutputReserve: 1024}
	b.SystemTokens = 500
	// budget would be negative, should clamp to 256
	got := b.HistoryBudget()
	if got != 256 {
		t.Errorf("HistoryBudget() = %d, want 256 (minimum)", got)
	}
}

func TestClassify(t *testing.T) {
	b := &ContextBudget{Window: 4096, OutputReserve: 1024}
	b.SystemTokens = 500
	b.RAGTokens = 500
	b.MemoryTokens = 100
	// HistoryBudget = 4096 - 1024 - 500 - 500 - 100 = 1972

	tests := []struct {
		name    string
		tokens  int
		want    ContextPressure
	}{
		{"normal_10%", 197, PressureNormal},
		{"normal_50%", 986, PressureNormal},
		{"warn_60%", 1184, PressureWarn},
		{"warn_70%", 1380, PressureWarn},
		{"critical_80%", 1578, PressureCritical},
		{"critical_90%", 1775, PressureCritical},
		{"emergency_95%", 1874, PressureEmergency},
		{"emergency_100%", 1972, PressureEmergency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.Classify(tt.tokens)
			if got != tt.want {
				t.Errorf("Classify(%d) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}
}

func TestManage_Normal(t *testing.T) {
	b := &ContextBudget{Window: 4096, OutputReserve: 1024}
	b.SystemTokens = 500
	b.RAGTokens = 500
	b.MemoryTokens = 100

	msgs := []store.Message{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好！有什么可以帮你的吗？"},
	}
	managed := b.Manage(msgs, nil, nil)
	if managed.Pressure != PressureNormal {
		t.Errorf("expected normal pressure, got %v", managed.Pressure)
	}
	if len(managed.ActionsTaken) != 0 {
		t.Errorf("expected no actions, got %v", managed.ActionsTaken)
	}
}

func TestManage_L1Trim(t *testing.T) {
	b := &ContextBudget{Window: 4096, OutputReserve: 1024}
	b.SystemTokens = 500
	b.RAGTokens = 500
	b.MemoryTokens = 100
	// HistoryBudget = 1972

	// Create a message > 500 chars that pushes past 60%
	longContent := strings.Repeat("这是一段很长的消息内容", 100) // ~1000 chars ≈ 1500 tokens
	msgs := []store.Message{
		{Role: "user", Content: "问题"},
		{Role: "assistant", Content: longContent},
	}

	managed := b.Manage(msgs, nil, nil)
	found := false
	for _, a := range managed.ActionsTaken {
		if a == "L1:trim_verbose" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected L1:trim_verbose in actions, got %v", managed.ActionsTaken)
	}
	// Verify the long message was trimmed
	for _, m := range managed.Messages {
		if len([]rune(m.Content)) > 500 {
			t.Errorf("message not trimmed, length = %d", len([]rune(m.Content)))
		}
	}
}

func TestManage_L2Merge(t *testing.T) {
	b := &ContextBudget{Window: 2048, OutputReserve: 512}
	b.SystemTokens = 300
	b.RAGTokens = 300
	b.MemoryTokens = 100
	// HistoryBudget = 2048 - 512 - 300 - 300 - 100 = 836

	// Build messages that push past 80%
	msgs := []store.Message{
		{Role: "user", Content: strings.Repeat("问题一内容", 50)},
		{Role: "assistant", Content: strings.Repeat("回答一内容很长很长", 50)},
		{Role: "user", Content: strings.Repeat("问题二内容", 50)},
		{Role: "assistant", Content: strings.Repeat("回答二内容很长很长", 50)},
		{Role: "user", Content: strings.Repeat("问题三内容", 50)},
		{Role: "assistant", Content: strings.Repeat("回答三内容很长很长", 50)},
	}

	managed := b.Manage(msgs, nil, nil)
	found := false
	for _, a := range managed.ActionsTaken {
		if a == "L2:merge_old_pairs" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected L2:merge_old_pairs in actions, got %v", managed.ActionsTaken)
	}
	// Should have fewer messages after merge
	if len(managed.Messages) >= len(msgs) {
		t.Errorf("expected fewer messages after merge: got %d, was %d", len(managed.Messages), len(msgs))
	}
}

func TestManage_L3Compress(t *testing.T) {
	b := &ContextBudget{Window: 1024, OutputReserve: 256}
	b.SystemTokens = 200
	b.RAGTokens = 200
	b.MemoryTokens = 50
	// HistoryBudget = 1024 - 256 - 200 - 200 - 50 = 318

	// Build messages that push past 95%
	msgs := []store.Message{
		{Role: "user", Content: strings.Repeat("很长的问题内容", 100)},
		{Role: "assistant", Content: strings.Repeat("很长的回答内容", 100)},
		{Role: "user", Content: strings.Repeat("第二轮问题", 100)},
		{Role: "assistant", Content: strings.Repeat("第二轮回答", 100)},
	}

	compressCalled := false
	mockCompress := func(msgs []store.Message) (string, error) {
		compressCalled = true
		return "这是压缩后的摘要", nil
	}

	managed := b.Manage(msgs, nil, mockCompress)
	if !compressCalled {
		t.Error("expected LLM compress to be called")
	}
	found := false
	for _, a := range managed.ActionsTaken {
		if a == "L3:llm_compress" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected L3:llm_compress in actions, got %v", managed.ActionsTaken)
	}
	// Should have compressed memory added
	hasCompressed := false
	for _, m := range managed.Memories {
		if m.Type == "compressed" {
			hasCompressed = true
			break
		}
	}
	if !hasCompressed {
		t.Error("expected compressed memory to be added")
	}
	// Should keep only last 2 messages
	if len(managed.Messages) > 2 {
		t.Errorf("expected at most 2 messages after L3, got %d", len(managed.Messages))
	}
}

func TestManage_L3Fallback(t *testing.T) {
	b := &ContextBudget{Window: 1024, OutputReserve: 256}
	b.SystemTokens = 200
	b.RAGTokens = 200
	b.MemoryTokens = 50

	msgs := []store.Message{
		{Role: "user", Content: strings.Repeat("问题", 200)},
		{Role: "assistant", Content: strings.Repeat("回答", 200)},
		{Role: "user", Content: strings.Repeat("问题", 200)},
		{Role: "assistant", Content: strings.Repeat("回答", 200)},
		{Role: "user", Content: strings.Repeat("问题", 200)},
		{Role: "assistant", Content: strings.Repeat("回答", 200)},
		{Role: "user", Content: strings.Repeat("问题", 200)},
		{Role: "assistant", Content: strings.Repeat("回答", 200)},
		{Role: "user", Content: strings.Repeat("问题", 200)},
		{Role: "assistant", Content: strings.Repeat("回答", 200)},
	}

	// LLM compress fails
	failCompress := func(msgs []store.Message) (string, error) {
		return "", fmt.Errorf("LLM unavailable")
	}

	managed := b.Manage(msgs, nil, failCompress)
	found := false
	for _, a := range managed.ActionsTaken {
		if a == "L3:fallback_truncate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected L3:fallback_truncate in actions, got %v", managed.ActionsTaken)
	}
	// Should keep at most 8 messages
	if len(managed.Messages) > 8 {
		t.Errorf("expected at most 8 messages after fallback, got %d", len(managed.Messages))
	}
}

func TestEstimateMemoryTokens(t *testing.T) {
	memories := []store.Memory{
		{Type: "short_term", Summary: "用户喜欢深色主题"},
		{Type: "long_term", Summary: "用户是高级开发者"},
	}
	got := EstimateMemoryTokens(memories)
	if got < 10 {
		t.Errorf("EstimateMemoryTokens too low: %d", got)
	}
}

func TestTrimVerboseMessages(t *testing.T) {
	msgs := []store.Message{
		{Role: "user", Content: "短消息"},
		{Role: "assistant", Content: strings.Repeat("很长的回答", 200)},
	}
	result := trimVerboseMessages(msgs)
	if len([]rune(result[0].Content)) != len([]rune("短消息")) {
		t.Error("short message should not be trimmed")
	}
	if len([]rune(result[1].Content)) > 305 { // 300 + "..."
		t.Errorf("long message not trimmed properly, got %d chars", len([]rune(result[1].Content)))
	}
}

func TestMergeOldPairs(t *testing.T) {
	now := time.Now()
	msgs := []store.Message{
		{Role: "user", Content: "第一个问题", CreatedAt: now},
		{Role: "assistant", Content: "第一个回答", CreatedAt: now},
		{Role: "user", Content: "第二个问题", CreatedAt: now},
		{Role: "assistant", Content: "第二个回答", CreatedAt: now},
		{Role: "user", Content: "第三个问题", CreatedAt: now},
		{Role: "assistant", Content: "第三个回答", CreatedAt: now},
	}
	result := mergeOldPairs(msgs)
	if len(result) != len(msgs)-1 {
		t.Errorf("expected %d messages after merge, got %d", len(msgs)-1, len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("merged message should have role 'system', got %q", result[0].Role)
	}
	if !strings.Contains(result[0].Content, "历史摘要") {
		t.Error("merged message should contain '历史摘要'")
	}
}

func TestMergeOldPairs_NotEnough(t *testing.T) {
	msgs := []store.Message{
		{Role: "user", Content: "问题"},
		{Role: "assistant", Content: "回答"},
	}
	result := mergeOldPairs(msgs)
	if len(result) != len(msgs) {
		t.Error("should not merge when <= 4 messages")
	}
}
