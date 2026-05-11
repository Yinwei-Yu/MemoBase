package core

import (
	"strings"
	"testing"

	"memobase/backend/internal/store"
)

func TestSplitIntoChunks_Unicode(t *testing.T) {
	t.Parallel()

	text := "你好世界测试文本"
	chunks := splitIntoChunks(text, 4, 0)
	if len(chunks) != 2 {
		t.Fatalf("len = %d; want 2", len(chunks))
	}
	if chunks[0] != "你好世界" {
		t.Fatalf("chunks[0] = %q; want %q", chunks[0], "你好世界")
	}
	if chunks[1] != "测试文本" {
		t.Fatalf("chunks[1] = %q; want %q", chunks[1], "测试文本")
	}
}

func TestSplitIntoChunks_NegativeChunkSize(t *testing.T) {
	t.Parallel()

	chunks := splitIntoChunks("hello", -1, 0)
	// Should default to 500
	if len(chunks) != 1 {
		t.Fatalf("len = %d; want 1", len(chunks))
	}
}

func TestSplitIntoChunks_NegativeOverlap(t *testing.T) {
	t.Parallel()

	chunks := splitIntoChunks("abcdefgh", 4, -5)
	if len(chunks) != 2 {
		t.Fatalf("len = %d; want 2", len(chunks))
	}
}

func TestSplitIntoChunks_WhitespaceOnly(t *testing.T) {
	t.Parallel()

	chunks := splitIntoChunks("   \n\t  ", 100, 10)
	if len(chunks) != 0 {
		t.Fatalf("len = %d; want 0", len(chunks))
	}
}

func TestSplitIntoChunks_SingleChar(t *testing.T) {
	t.Parallel()

	chunks := splitIntoChunks("a", 1, 0)
	if len(chunks) != 1 || chunks[0] != "a" {
		t.Fatalf("chunks = %v; want [a]", chunks)
	}
}

func TestTokenize_EmptyString(t *testing.T) {
	t.Parallel()

	got := tokenize("")
	if len(got) != 0 {
		t.Fatalf("len = %d; want 0", len(got))
	}
}

func TestTokenize_OnlyPunctuation(t *testing.T) {
	t.Parallel()

	got := tokenize("!@#$%^&*()")
	if len(got) != 0 {
		t.Fatalf("len = %d; want 0", len(got))
	}
}

func TestTokenize_MixedLanguages(t *testing.T) {
	t.Parallel()

	got := tokenize("Hello 你好 World")
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3; got=%v", len(got), got)
	}
}

func TestBM25LikeScore_EmptyDoc(t *testing.T) {
	t.Parallel()

	score := bm25LikeScore("query", "")
	if score != 0 {
		t.Fatalf("score = %f; want 0", score)
	}
}

func TestBM25LikeScore_BothEmpty(t *testing.T) {
	t.Parallel()

	score := bm25LikeScore("", "")
	if score != 0 {
		t.Fatalf("score = %f; want 0", score)
	}
}

func TestBM25LikeScore_MultipleMatches(t *testing.T) {
	t.Parallel()

	score1 := bm25LikeScore("thread", "thread process")
	score2 := bm25LikeScore("thread", "thread thread thread process")
	if score2 <= score1 {
		t.Fatalf("more matches should score higher: %f vs %f", score1, score2)
	}
}

func TestNormalizeScores_SingleEntry(t *testing.T) {
	t.Parallel()

	got := normalizeScores(map[string]float64{"a": 5})
	if got["a"] != 1 {
		t.Fatalf("single entry should normalize to 1; got %f", got["a"])
	}
}

func TestBuildChatPrompt_NoMemories(t *testing.T) {
	t.Parallel()

	app := &App{}
	prompt := app.BuildChatPrompt(
		"question",
		[]RetrievedChunk{{Chunk: store.Chunk{ID: "ck1", Content: "content"}}},
		nil,
	)
	if !strings.Contains(prompt, "[上下文片段]") {
		t.Fatalf("prompt missing context section")
	}
	if strings.Contains(prompt, "[记忆]") {
		t.Fatalf("prompt should not contain memory section when no memories")
	}
}

func TestBuildChatPrompt_EmptyChunks(t *testing.T) {
	t.Parallel()

	app := &App{}
	prompt := app.BuildChatPrompt("question", nil, nil)
	if !strings.Contains(prompt, "[用户问题]") {
		t.Fatalf("prompt missing question section")
	}
}

func TestBuildChatPrompt_WithMemory(t *testing.T) {
	t.Parallel()

	app := &App{}
	prompt := app.BuildChatPrompt(
		"test",
		nil,
		[]store.Memory{
			{Summary: "用户喜欢Go语言"},
			{Summary: "用户在准备考试"},
		},
	)
	if !strings.Contains(prompt, "用户喜欢Go语言") {
		t.Fatalf("prompt missing first memory")
	}
	if !strings.Contains(prompt, "用户在准备考试") {
		t.Fatalf("prompt missing second memory")
	}
}

func TestQdrantCollectionForKB_SpecialChars(t *testing.T) {
	t.Parallel()

	app := &App{}
	tests := []struct {
		kbID string
		want string
	}{
		{"kb_test", "kb_chunks__kb_test"},
		{"KB/with/slashes", "kb_chunks__KB_with_slashes"},
		{"  spaces  ", "kb_chunks__spaces"},
		{"!@#$%", "kb_chunks__default"},
		{"a", "kb_chunks__a"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.kbID, func(t *testing.T) {
			t.Parallel()
			got := app.QdrantCollectionForKB(tt.kbID)
			if got != tt.want {
				t.Fatalf("QdrantCollectionForKB(%q) = %q; want %q", tt.kbID, got, tt.want)
			}
		})
	}
}

func TestSummarize_EmptyString(t *testing.T) {
	t.Parallel()

	got := summarize("", 10)
	if got != "" {
		t.Fatalf("summarize('') = %q; want empty", got)
	}
}

func TestSummarize_ExactLength(t *testing.T) {
	t.Parallel()

	got := summarize("hello", 5)
	if got != "hello" {
		t.Fatalf("summarize('hello', 5) = %q; want 'hello'", got)
	}
}

func TestSummarize_UnicodeTruncation(t *testing.T) {
	t.Parallel()

	got := summarize("你好世界测试", 4)
	if got != "你好世界" {
		t.Fatalf("summarize = %q; want '你好世界'", got)
	}
}

func TestSanitizeQdrantCollectionPart_OnlyUnderscores(t *testing.T) {
	t.Parallel()

	got := sanitizeQdrantCollectionPart("___test___")
	if got != "test" {
		t.Fatalf("sanitizeQdrantCollectionPart('___test___') = %q; want 'test'", got)
	}
}
