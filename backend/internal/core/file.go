package core

import (
	"io"
	"os"
	"path/filepath"
)

func SaveUploadedFile(storageDir, kbID, docID, fileName string, src io.Reader) (string, error) {
	dir := filepath.Join(storageDir, kbID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, docID+"_"+fileName)
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
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
