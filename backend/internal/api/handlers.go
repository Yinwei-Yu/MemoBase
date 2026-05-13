package api

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

var supportedUploadExt = map[string]struct{}{
	".txt": {},
	".md":  {},
}

const (
	maxUploadFileSize  = 20 * 1024 * 1024
	maxUploadFileCount = 20
)

type uploadDocumentItem struct {
	DocID     string `json:"doc_id"`
	KBID      string `json:"kb_id"`
	FileName  string `json:"file_name"`
	Status    string `json:"status"`
	TaskID    string `json:"task_id"`
	CreatedAt any    `json:"created_at"`
}

type uploadDocumentsResponse struct {
	Items         []uploadDocumentItem `json:"items"`
	UploadedCount int                  `json:"uploaded_count"`
}

func isSupportedUploadExt(ext string) bool {
	_, ok := supportedUploadExt[strings.ToLower(strings.TrimSpace(ext))]
	return ok
}

func parsePage(c *gin.Context) (int, int, int) {
	page := 1
	pageSize := 20
	if v := c.Query("page"); v != "" {
		if n, err := parsePositiveInt(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := c.Query("page_size"); v != "" {
		if n, err := parsePositiveInt(v); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	return page, pageSize, (page - 1) * pageSize
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("not a positive integer")
	}
	return n, nil
}

func userIDFrom(c *gin.Context) string {
	uid, _ := c.Get("user_id")
	s, _ := uid.(string)
	return s
}

func trimAndFilterTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		out = append(out, tag)
	}
	return out
}

func validateKBFields(name, description *string, tags *[]string) error {
	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return fmt.Errorf("name is required")
		}
		if utf8.RuneCountInString(trimmed) > 64 {
			return fmt.Errorf("name must be between 1 and 64 characters")
		}
		*name = trimmed
	}
	if description != nil {
		trimmed := strings.TrimSpace(*description)
		if utf8.RuneCountInString(trimmed) > 512 {
			return fmt.Errorf("description must be at most 512 characters")
		}
		*description = trimmed
	}
	if tags != nil {
		filtered := trimAndFilterTags(*tags)
		if len(filtered) > 10 {
			return fmt.Errorf("tags must be at most 10 items")
		}
		*tags = filtered
	}
	return nil
}

func clampTopK(v int) int {
	if v <= 0 {
		return 6
	}
	if v > 20 {
		return 20
	}
	return v
}

func coreSummary(text string, n int) string {
	r := []rune(strings.TrimSpace(text))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}
