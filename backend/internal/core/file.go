package core

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

func SaveUploadedFile(storageDir, kbID, docID, fileName string, src io.Reader) (string, error) {
	dir := filepath.Join(storageDir, kbID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Sanitize fileName to prevent path traversal: strip directory components
	cleanName := filepath.Base(fileName)
	if cleanName == "." || cleanName == "/" {
		cleanName = "unnamed"
	}
	target := filepath.Join(dir, docID+"_"+cleanName)
	file, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, src); err != nil {
		return "", err
	}
	return target, nil
}

func readTextFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return detectAndDecode(raw)
}

// detectAndDecode attempts to decode raw bytes into a UTF-8 string.
// Detection order: BOM → UTF-8 → GB18030 → Big5 → raw UTF-8 fallback.
func detectAndDecode(raw []byte) (string, error) {
	// Strip BOM
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}

	// Fast path: valid UTF-8
	if utf8.Valid(raw) {
		return string(raw), nil
	}

	// Try GB18030 (superset of GBK/GB2312)
	if decoded, err := decodeWith(raw, simplifiedchinese.GB18030.NewDecoder()); err == nil && utf8.ValidString(decoded) {
		return decoded, nil
	}

	// Try Big5
	if decoded, err := decodeWith(raw, traditionalchinese.Big5.NewDecoder()); err == nil && utf8.ValidString(decoded) {
		return decoded, nil
	}

	// Fallback: return as-is (will contain replacement characters)
	return string(raw), nil
}

func decodeWith(raw []byte, dec transform.Transformer) (string, error) {
	reader := transform.NewReader(bytes.NewReader(raw), dec)
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
