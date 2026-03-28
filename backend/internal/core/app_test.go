package core

import (
	"math"
	"testing"

	"memobase/backend/internal/config"

	"github.com/google/uuid"
)

func TestSplitIntoChunks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		chunkSize int
		overlap   int
		want      []string
	}{
		{
			name:      "empty text",
			text:      "",
			chunkSize: 500,
			overlap:   100,
			want:      []string{},
		},
		{
			name:      "no overlap",
			text:      "abcdef",
			chunkSize: 3,
			overlap:   0,
			want:      []string{"abc", "def"},
		},
		{
			name:      "overlap enabled",
			text:      "abcdefghi",
			chunkSize: 5,
			overlap:   1,
			want:      []string{"abcde", "efghi"},
		},
		{
			name:      "overlap larger than chunk resets",
			text:      "abcdefghi",
			chunkSize: 5,
			overlap:   6,
			want:      []string{"abcde", "efghi"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := splitIntoChunks(tt.text, tt.chunkSize, tt.overlap)
			if len(got) != len(tt.want) {
				t.Fatalf("splitIntoChunks() len = %d; want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitIntoChunks()[%d] = %q; want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNormalizeScores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  map[string]float64
		assert func(t *testing.T, got map[string]float64)
	}{
		{
			name:  "empty map",
			input: map[string]float64{},
			assert: func(t *testing.T, got map[string]float64) {
				t.Helper()
				if len(got) != 0 {
					t.Fatalf("len(got) = %d; want 0", len(got))
				}
			},
		},
		{
			name:  "all values equal",
			input: map[string]float64{"a": 10, "b": 10},
			assert: func(t *testing.T, got map[string]float64) {
				t.Helper()
				if got["a"] != 1 || got["b"] != 1 {
					t.Fatalf("equal-score normalization = %#v; want all ones", got)
				}
			},
		},
		{
			name:  "min max scaling",
			input: map[string]float64{"a": 2, "b": 6, "c": 10},
			assert: func(t *testing.T, got map[string]float64) {
				t.Helper()
				assertNearlyEqual(t, got["a"], 0)
				assertNearlyEqual(t, got["b"], 0.5)
				assertNearlyEqual(t, got["c"], 1)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeScores(tt.input)
			tt.assert(t, got)
		})
	}
}

func TestBM25LikeScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		doc   string
		check func(t *testing.T, score float64)
	}{
		{
			name:  "empty query returns zero",
			query: "",
			doc:   "thread process",
			check: func(t *testing.T, score float64) {
				t.Helper()
				if score != 0 {
					t.Fatalf("score = %f; want 0", score)
				}
			},
		},
		{
			name:  "no matching tokens returns zero",
			query: "network",
			doc:   "thread process",
			check: func(t *testing.T, score float64) {
				t.Helper()
				if score != 0 {
					t.Fatalf("score = %f; want 0", score)
				}
			},
		},
		{
			name:  "matching token returns positive",
			query: "thread",
			doc:   "thread process thread",
			check: func(t *testing.T, score float64) {
				t.Helper()
				if score <= 0 {
					t.Fatalf("score = %f; want > 0", score)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			score := bm25LikeScore(tt.query, tt.doc)
			tt.check(t, score)
		})
	}
}

func assertNearlyEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %f; want %f", got, want)
	}
}

func TestQdrantPointID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chunkID string
	}{
		{name: "uuid input", chunkID: uuid.NewString()},
		{name: "prefixed id", chunkID: "ck_" + uuid.NewString()},
		{name: "plain text", chunkID: "chunk-1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := qdrantPointID(tt.chunkID)
			if _, err := uuid.Parse(got); err != nil {
				t.Fatalf("qdrantPointID(%q) = %q; not a valid UUID: %v", tt.chunkID, got, err)
			}
			again := qdrantPointID(tt.chunkID)
			if got != again {
				t.Fatalf("qdrantPointID(%q) not deterministic: %q vs %q", tt.chunkID, got, again)
			}
		})
	}
}

func TestQdrantCollectionForKB(t *testing.T) {
	t.Parallel()

	app := &App{
		Config: config.Config{
			QdrantCollection: "kb_chunks",
		},
	}

	tests := []struct {
		name string
		kbID string
		want string
	}{
		{
			name: "normal kb id",
			kbID: "kb_123",
			want: "kb_chunks__kb_123",
		},
		{
			name: "sanitize invalid chars",
			kbID: "kb/with space",
			want: "kb_chunks__kb_with_space",
		},
		{
			name: "empty kb id fallback",
			kbID: "",
			want: "kb_chunks__default",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := app.QdrantCollectionForKB(tt.kbID)
			if got != tt.want {
				t.Fatalf("QdrantCollectionForKB(%q) = %q; want %q", tt.kbID, got, tt.want)
			}
		})
	}
}
