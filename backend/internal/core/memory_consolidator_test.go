package core

import (
	"encoding/json"
	"testing"
)

func TestNormalizeSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		maxLen int
	}{
		{"normal", "Hello World", 30},
		{"trimmed", "  trimmed  ", 30},
		{"empty", "", 30},
		{"long", string(make([]byte, 100)), 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSummary(tt.input)
			if len(got) > tt.maxLen {
				t.Errorf("normalizeSummary(%q) too long: %d chars", tt.input, len(got))
			}
			if tt.name == "normal" && got != "hello world" {
				t.Errorf("got %q; want %q", got, "hello world")
			}
			if tt.name == "trimmed" && got != "trimmed" {
				t.Errorf("got %q; want %q", got, "trimmed")
			}
		})
	}
}

func TestNormalizeSummary_DedupKey(t *testing.T) {
	t.Parallel()

	// Same 30-char prefix, different suffix — should produce same dedup key
	a := normalizeSummary("用户在对话中多次询问关于Go语言并发编程的性能优化问题解答和最佳实践方案讨论")
	b := normalizeSummary("用户在对话中多次询问关于Go语言并发编程的性能优化问题解答和表述方式略有不同")
	if a != b {
		t.Errorf("expected same dedup key: %q vs %q", a, b)
	}

	// Different content should produce different key
	c := normalizeSummary("用户询问Python数据科学库的使用方法和具体示例代码")
	if a == c {
		t.Error("expected different dedup keys for different summaries")
	}
}

func TestExtractedMemory_JSON(t *testing.T) {
	t.Parallel()

	jsonStr := `[{"type":"long_term","summary":"用户熟悉Go语言","importance":0.8},{"type":"fact","summary":"公司用K8s部署","importance":0.9}]`

	var extracted []extractedMemory
	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(extracted) != 2 {
		t.Fatalf("expected 2 items, got %d", len(extracted))
	}
	if extracted[0].Type != "long_term" {
		t.Errorf("type = %s; want long_term", extracted[0].Type)
	}
	if extracted[0].Importance != 0.8 {
		t.Errorf("importance = %f; want 0.8", extracted[0].Importance)
	}
	if extracted[1].Type != "fact" {
		t.Errorf("type = %s; want fact", extracted[1].Type)
	}
}

func TestExtractedMemory_EmptyArray(t *testing.T) {
	t.Parallel()

	jsonStr := `[]`
	var extracted []extractedMemory
	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(extracted) != 0 {
		t.Errorf("expected 0 items, got %d", len(extracted))
	}
}

func TestProviderConfig_Fields(t *testing.T) {
	t.Parallel()

	p := providerConfig{
		baseURL: "http://localhost:11434",
		apiKey:  "test-key",
		model:   "qwen2.5:3b",
	}
	if p.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %s", p.baseURL)
	}
	if p.model != "qwen2.5:3b" {
		t.Errorf("model = %s", p.model)
	}
}
