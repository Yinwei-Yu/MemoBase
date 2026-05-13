package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveUploadedFileAndReadTextFile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	content := "hello memobase"
	path, err := SaveUploadedFile(base, "kb_1", "doc_1", "test.md", strings.NewReader(content))
	if err != nil {
		t.Fatalf("SaveUploadedFile() error = %v", err)
	}
	if filepath.Base(path) != "doc_1_test.md" {
		t.Fatalf("saved filename = %q; want %q", filepath.Base(path), "doc_1_test.md")
	}

	got, err := readTextFile(path)
	if err != nil {
		t.Fatalf("readTextFile() error = %v", err)
	}
	if got != content {
		t.Fatalf("readTextFile() = %q; want %q", got, content)
	}
}

func TestReadTextFileMissing(t *testing.T) {
	t.Parallel()
	_, err := readTextFile(filepath.Join(t.TempDir(), "missing.txt"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("error = %v; want os.IsNotExist", err)
	}
}

func TestDetectAndDecode_UTF8(t *testing.T) {
	t.Parallel()
	input := []byte("你好世界 hello")
	got, err := detectAndDecode(input)
	if err != nil {
		t.Fatalf("detectAndDecode() error = %v", err)
	}
	if got != "你好世界 hello" {
		t.Fatalf("detectAndDecode() = %q; want %q", got, "你好世界 hello")
	}
}

func TestDetectAndDecode_UTF8BOM(t *testing.T) {
	t.Parallel()
	input := []byte{0xEF, 0xBB, 0xBF, 0xE4, 0xBD, 0xA0, 0xE5, 0xA5, 0xBD} // BOM + "你好"
	got, err := detectAndDecode(input)
	if err != nil {
		t.Fatalf("detectAndDecode() error = %v", err)
	}
	if got != "你好" {
		t.Fatalf("detectAndDecode() = %q; want %q", got, "你好")
	}
}

func TestDetectAndDecode_GB18030(t *testing.T) {
	t.Parallel()
	// "你好" in GB18030 encoding: 0xC4, 0xE3, 0xBA, 0xC3
	input := []byte{0xC4, 0xE3, 0xBA, 0xC3}
	got, err := detectAndDecode(input)
	if err != nil {
		t.Fatalf("detectAndDecode() error = %v", err)
	}
	if got != "你好" {
		t.Fatalf("detectAndDecode() = %q; want %q", got, "你好")
	}
}
