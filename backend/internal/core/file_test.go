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
