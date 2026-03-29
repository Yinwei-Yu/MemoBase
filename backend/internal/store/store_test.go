package store

import (
	"database/sql"
	"testing"
)

func TestStoreIsNotFound(t *testing.T) {
	t.Parallel()

	if !IsNotFound(sql.ErrNoRows) {
		t.Fatalf("IsNotFound(sql.ErrNoRows) = false; want true")
	}
	if IsNotFound(nil) {
		t.Fatalf("IsNotFound(nil) = true; want false")
	}
	if IsNotFound(sql.ErrConnDone) {
		t.Fatalf("IsNotFound(sql.ErrConnDone) = true; want false")
	}
}
