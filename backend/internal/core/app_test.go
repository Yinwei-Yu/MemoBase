package core

import (
	"math"
	"testing"

	"memobase/backend/internal/config"

	"github.com/google/uuid"
)

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
