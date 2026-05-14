package core

import "testing"

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
