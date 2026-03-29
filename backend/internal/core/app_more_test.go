package core

import (
	"strings"
	"testing"

	"memobase/backend/internal/store"
)

func TestSanitizeQdrantCollectionPart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "kb_chunks", want: "kb_chunks"},
		{in: " kb/with space ", want: "kb_with_space"},
		{in: "!!!", want: "default"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeQdrantCollectionPart(tt.in); got != tt.want {
				t.Fatalf("sanitizeQdrantCollectionPart(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	t.Parallel()

	got := tokenize("Thread-1 与 进程 process_2!")
	want := []string{"thread", "1", "与", "进程", "process", "2"}
	if len(got) != len(want) {
		t.Fatalf("len(tokenize()) = %d; want %d; got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tokenize()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestSummarize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		n    int
		want string
	}{
		{name: "short text", text: "hello", n: 10, want: "hello"},
		{name: "trim spaces", text: "  hello ", n: 10, want: "hello"},
		{name: "truncate with ellipsis", text: "hello world", n: 8, want: "hello..."},
		{name: "small max keeps prefix", text: "abcdef", n: 3, want: "abc"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := summarize(tt.text, tt.n); got != tt.want {
				t.Fatalf("summarize(%q, %d) = %q; want %q", tt.text, tt.n, got, tt.want)
			}
		})
	}
}

func TestBuildChatPrompt(t *testing.T) {
	t.Parallel()

	app := &App{}
	prompt := app.BuildChatPrompt(
		"问题是什么？",
		[]RetrievedChunk{
			{Chunk: store.Chunk{ID: "ck1", Content: "chunk one"}},
			{Chunk: store.Chunk{ID: "ck2", Content: "chunk two"}},
		},
		[]store.Memory{
			{Summary: "memory summary"},
		},
	)

	mustContain := []string{
		"[上下文片段]",
		"[1] (ck1) chunk one",
		"[2] (ck2) chunk two",
		"[记忆]",
		"memory summary",
		"[用户问题]",
		"问题是什么？",
	}
	for _, s := range mustContain {
		if !strings.Contains(prompt, s) {
			t.Fatalf("prompt missing %q:\n%s", s, prompt)
		}
	}
}
