package core

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
		tol   int // tolerance (absolute)
	}{
		{"empty", "", 0, 0},
		{"pure_chinese_short", "你好世界", 6, 2},
		{"pure_chinese_long", strings.Repeat("中", 100), 150, 30},
		{"pure_english_short", "hello world", 3, 2},
		{"pure_english_long", "the quick brown fox jumps over the lazy dog", 17, 5},
		{"mixed", "hello你好world", 6, 3},
		{"digits", "12345", 2, 2},
		{"punctuation", "!@#$%", 3, 2},
		{"code_like", "func main() { fmt.Println(\"hello\") }", 10, 5},
		{"chinese_sentence", "今天天气真好，我们去公园散步吧", 22, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tol {
				t.Errorf("EstimateTokens(%q) = %d, want %d (tol %d)", tt.input, got, tt.want, tt.tol)
			}
		})
	}
}

func TestIsCJK(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want bool
	}{
		{"chinese_char", '中', true},
		{"chinese_char2", '你', true},
		{"english_char", 'a', false},
		{"digit", '1', false},
		{"space", ' ', false},
		{"cjk_ext_a", 0x3400, true},
		{"cjk_compat", 0xF900, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCJK(tt.r)
			if got != tt.want {
				t.Errorf("isCJK(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	msgs := []MessageTokenInput{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好！有什么可以帮你的吗？"},
	}
	got := EstimateMessagesTokens(msgs)
	if got < 10 {
		t.Errorf("EstimateMessagesTokens too low: %d", got)
	}
}

func TestEstimateTokens_Monotonic(t *testing.T) {
	short := EstimateTokens("hello")
	long := EstimateTokens("hello world foo bar baz qux quux")
	if long <= short {
		t.Errorf("longer text should have more tokens: short=%d long=%d", short, long)
	}
}

func TestEstimateTokens_NoPanic(t *testing.T) {
	// edge cases that shouldn't panic
	EstimateTokens(" ")
	EstimateTokens("\n\t\r")
	EstimateTokens(string(rune(0)))
	EstimateTokens(strings.Repeat("a", 10000))
}
