package api

import (
	"net/http"
	"net/http/httptest"
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

func newQueryContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test?"+rawQuery, nil)
	c.Request = req
	return c
}
