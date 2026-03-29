package infra

import (
	"database/sql"
	"testing"
)

func TestInfraIsNotFound(t *testing.T) {
	t.Parallel()
	if !IsNotFound(sql.ErrNoRows) {
		t.Fatalf("IsNotFound(sql.ErrNoRows) = false; want true")
	}
	if IsNotFound(nil) {
		t.Fatalf("IsNotFound(nil) = true; want false")
	}
}
