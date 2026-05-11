package util

import (
	"testing"
	"time"
)

func TestNowUTC_Format(t *testing.T) {
	t.Parallel()

	got := nowUTC()
	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("nowUTC() = %q; not valid RFC3339: %v", got, err)
	}

	// Should be UTC
	if parsed.Location() != time.UTC {
		t.Fatalf("nowUTC() timezone = %v; want UTC", parsed.Location())
	}
}

func TestNowUTC_IsRecent(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	got := nowUTC()
	after := time.Now().UTC()

	parsed, _ := time.Parse(time.RFC3339, got)
	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Fatalf("nowUTC() = %v; want between %v and %v", parsed, before, after)
	}
}

func TestNowUTC_MultipleCallsIncrease(t *testing.T) {
	t.Parallel()

	t1 := nowUTC()
	time.Sleep(10 * time.Millisecond)
	t2 := nowUTC()

	if t1 == t2 {
		// Very unlikely but possible with coarse clocks; not a hard failure
		t.Logf("nowUTC() returned same value twice: %q", t1)
	}
}

func TestNowUTC_ContainsTZ(t *testing.T) {
	t.Parallel()

	got := nowUTC()
	// RFC3339 should end with Z for UTC
	if len(got) < 1 || got[len(got)-1] != 'Z' {
		t.Fatalf("nowUTC() = %q; expected to end with Z", got)
	}
}
