package core

import (
	"fmt"

	"memobase/backend/internal/store"
)

// ContextPressure represents how much of the context window is used.
type ContextPressure int

const (
	PressureNormal    ContextPressure = iota // < 60% HistoryBudget
	PressureWarn                             // 60-80%
	PressureCritical                         // 80-95%
	PressureEmergency                        // > 95%
)

func (p ContextPressure) String() string {
	switch p {
	case PressureWarn:
		return "warn"
	case PressureCritical:
		return "critical"
	case PressureEmergency:
		return "emergency"
	default:
		return "normal"
	}
}

// ContextBudget manages token allocation across context window.
type ContextBudget struct {
	Window        int // model context window
	OutputReserve int // reserved for output
	SystemTokens  int // system prompt estimate
	RAGTokens     int // RAG chunks estimate
	MemoryTokens  int // memories estimate
}

// NewContextBudget creates a budget with window and output reserve.
func NewContextBudget(window, outputReserve int) *ContextBudget {
	return &ContextBudget{
		Window:        window,
		OutputReserve: outputReserve,
	}
}

// HistoryBudget returns tokens available for history.
func (b *ContextBudget) HistoryBudget() int {
	budget := b.Window - b.OutputReserve - b.SystemTokens - b.RAGTokens - b.MemoryTokens
	if budget < 256 {
		budget = 256 // minimum guarantee
	}
	return budget
}

// Classify determines pressure level for given token count.
func (b *ContextBudget) Classify(currentTokens int) ContextPressure {
	budget := b.HistoryBudget()
	if budget <= 0 {
		return PressureEmergency
	}
	ratio := float64(currentTokens) / float64(budget)

	switch {
	case ratio >= 0.95:
		return PressureEmergency
	case ratio >= 0.80:
		return PressureCritical
	case ratio >= 0.60:
		return PressureWarn
	default:
		return PressureNormal
	}
}

// ManagedContext is the result of context management.
type ManagedContext struct {
	Messages     []store.Message
	Memories     []store.Memory
	TokenCount   int
	Pressure     ContextPressure
	ActionsTaken []string
}

// Manage applies tiered degradation to messages+memories.
func (b *ContextBudget) Manage(
	messages []store.Message,
	memories []store.Memory,
	llmCompressFn func([]store.Message) (string, error),
) ManagedContext {
	actions := []string{}

	total := b.estimateAll(messages, memories)
	pressure := b.Classify(total)

	if pressure == PressureNormal {
		return ManagedContext{
			Messages:     messages,
			Memories:     memories,
			TokenCount:   total,
			Pressure:     pressure,
			ActionsTaken: actions,
		}
	}

	// L1: trim verbose messages (>60%)
	if pressure >= PressureWarn {
		messages = trimVerboseMessages(messages)
		actions = append(actions, "L1:trim_verbose")
		total = b.estimateAll(messages, memories)
		pressure = b.Classify(total)
	}

	// L2: merge old message pairs (>80%)
	if pressure >= PressureCritical {
		messages = mergeOldPairs(messages)
		actions = append(actions, "L2:merge_old_pairs")
		total = b.estimateAll(messages, memories)
		pressure = b.Classify(total)
	}

	// L3: LLM compress (>95%)
	if pressure >= PressureEmergency && llmCompressFn != nil {
		compressed, err := llmCompressFn(messages)
		if err == nil && compressed != "" {
			memories = append(memories, store.Memory{
				Type:    "compressed",
				Summary: compressed,
			})
			if len(messages) > 2 {
				messages = messages[len(messages)-2:]
			}
			actions = append(actions, "L3:llm_compress")
		} else {
			// fallback: keep last 4 turns
			if len(messages) > 8 {
				messages = messages[len(messages)-8:]
			}
			actions = append(actions, "L3:fallback_truncate")
		}
		total = b.estimateAll(messages, memories)
		pressure = b.Classify(total)
	}

	return ManagedContext{
		Messages:     messages,
		Memories:     memories,
		TokenCount:   total,
		Pressure:     pressure,
		ActionsTaken: actions,
	}
}

func (b *ContextBudget) estimateAll(messages []store.Message, memories []store.Memory) int {
	total := 0
	for _, m := range messages {
		total += 4 + EstimateTokens(m.Content)
	}
	for _, m := range memories {
		total += 4 + EstimateTokens(m.Summary)
	}
	return total
}

// EstimateMemoryTokens estimates total tokens for a memory list.
func EstimateMemoryTokens(memories []store.Memory) int {
	total := 0
	for _, m := range memories {
		total += 4 + EstimateTokens(m.Summary)
	}
	return total
}

// L1: truncate messages longer than 500 chars to 300 chars.
func trimVerboseMessages(messages []store.Message) []store.Message {
	result := make([]store.Message, 0, len(messages))
	for _, m := range messages {
		if len([]rune(m.Content)) > 500 {
			runes := []rune(m.Content)
			m.Content = string(runes[:300]) + "..."
		}
		result = append(result, m)
	}
	return result
}

// L2: merge earliest 2 messages into a summary.
func mergeOldPairs(messages []store.Message) []store.Message {
	if len(messages) <= 4 {
		return messages
	}

	merged := store.Message{
		Role:      "system",
		Content:   fmt.Sprintf("[历史摘要] 用户: %s | 助手: %s", truncateStr(messages[0].Content, 60), truncateStr(messages[1].Content, 100)),
		CreatedAt: messages[0].CreatedAt,
	}

	result := make([]store.Message, 0, len(messages)-1)
	result = append(result, merged)
	result = append(result, messages[2:]...)
	return result
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
