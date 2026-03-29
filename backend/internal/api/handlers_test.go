package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsSupportedUploadExt(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		want bool
	}{
		{name: "txt", ext: ".txt", want: true},
		{name: "md", ext: ".md", want: true},
		{name: "uppercase", ext: ".TXT", want: true},
		{name: "trim spaces", ext: "  .md  ", want: true},
		{name: "pdf not supported", ext: ".pdf", want: false},
		{name: "empty", ext: "", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := isSupportedUploadExt(tt.ext)
			if got != tt.want {
				t.Fatalf("isSupportedUploadExt(%q) = %v; want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestParsePage(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantPage     int
		wantPageSize int
		wantOffset   int
	}{
		{name: "defaults", query: "", wantPage: 1, wantPageSize: 20, wantOffset: 0},
		{name: "custom values", query: "page=3&page_size=10", wantPage: 3, wantPageSize: 10, wantOffset: 20},
		{name: "invalid values fallback", query: "page=-1&page_size=abc", wantPage: 1, wantPageSize: 20, wantOffset: 0},
		{name: "max page size clamp", query: "page=2&page_size=500", wantPage: 2, wantPageSize: 100, wantOffset: 100},
		{name: "zero page size fallback", query: "page=2&page_size=0", wantPage: 2, wantPageSize: 20, wantOffset: 20},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := newQueryContext(tt.query)
			page, pageSize, offset := parsePage(c)

			if page != tt.wantPage || pageSize != tt.wantPageSize || offset != tt.wantOffset {
				t.Fatalf("parsePage(%q) = (%d,%d,%d); want (%d,%d,%d)",
					tt.query, page, pageSize, offset, tt.wantPage, tt.wantPageSize, tt.wantOffset)
			}
		})
	}
}

func TestCoreSummary(t *testing.T) {
	tests := []struct {
		name string
		text string
		n    int
		want string
	}{
		{name: "no truncation", text: "hello", n: 5, want: "hello"},
		{name: "trim spaces", text: "  hello  ", n: 10, want: "hello"},
		{name: "truncate ascii", text: "hello world", n: 5, want: "hello..."},
		{name: "truncate unicode runes", text: "你好世界", n: 3, want: "你好世..."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := coreSummary(tt.text, tt.n)
			if got != tt.want {
				t.Fatalf("coreSummary(%q, %d) = %q; want %q", tt.text, tt.n, got, tt.want)
			}
		})
	}
}

func TestValidateKBFields(t *testing.T) {
	tests := []struct {
		name        string
		inName      *string
		inDesc      *string
		inTags      *[]string
		wantErrPart string
	}{
		{
			name:   "valid create payload",
			inName: strPtr("  test kb  "),
			inDesc: strPtr("desc"),
			inTags: tagsPtr([]string{" tag1 ", "tag2", ""}),
		},
		{
			name:        "name too long",
			inName:      strPtr(strings.Repeat("a", 65)),
			wantErrPart: "name must be between 1 and 64 characters",
		},
		{
			name:        "description too long",
			inDesc:      strPtr(strings.Repeat("d", 513)),
			wantErrPart: "description must be at most 512 characters",
		},
		{
			name:        "too many tags",
			inTags:      tagsPtr([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}),
			wantErrPart: "tags must be at most 10 items",
		},
		{
			name:        "empty name invalid when provided",
			inName:      strPtr("   "),
			wantErrPart: "name is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := validateKBFields(tt.inName, tt.inDesc, tt.inTags)
			if tt.wantErrPart == "" {
				if err != nil {
					t.Fatalf("validateKBFields() error = %v; want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Fatalf("validateKBFields() error = %v; want contains %q", err, tt.wantErrPart)
			}
		})
	}
}

func TestClampTopK(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{in: -1, want: 6},
		{in: 0, want: 6},
		{in: 1, want: 1},
		{in: 6, want: 6},
		{in: 20, want: 20},
		{in: 99, want: 20},
	}

	for _, tt := range tests {
		if got := clampTopK(tt.in); got != tt.want {
			t.Fatalf("clampTopK(%d) = %d; want %d", tt.in, got, tt.want)
		}
	}
}

func TestTrimAndFilterTags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "trim and drop empty",
			in:   []string{" tag1 ", "", "  ", "tag2"},
			want: []string{"tag1", "tag2"},
		},
		{
			name: "keep order",
			in:   []string{"b", "a", "c"},
			want: []string{"b", "a", "c"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := trimAndFilterTags(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len(trimAndFilterTags()) = %d; want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("trimAndFilterTags()[%d] = %q; want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func newQueryContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test?"+rawQuery, nil)
	c.Request = req
	return c
}

func strPtr(v string) *string {
	return &v
}

func tagsPtr(v []string) *[]string {
	return &v
}
