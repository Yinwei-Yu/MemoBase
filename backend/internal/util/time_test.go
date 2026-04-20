package util

import (
	"testing"
	"time"
)

func TestNowUTC(t *testing.T) {
	t.Parallel()

	value := nowUTC()
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		t.Fatalf("nowUTC() = %q; parse error: %v", value, err)
	}
}
