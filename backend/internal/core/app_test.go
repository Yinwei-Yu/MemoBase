package core

import (
	"math"
	"testing"

	"memobase/backend/internal/config"
)

func assertNearlyEqual(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %f; want %f", got, want)
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
