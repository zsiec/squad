package items

import (
	"testing"
	"time"
)

func TestCounts_TalliesByCategory(t *testing.T) {
	w, err := Walk("testdata/ready")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	c := Counts(w, now)
	if c.InProgress != 0 {
		t.Fatalf("in_progress=%d want 0", c.InProgress)
	}
	if c.Ready < 1 {
		t.Fatalf("ready=%d want >=1", c.Ready)
	}
	if c.Blocked != 2 {
		t.Fatalf("blocked=%d want 2", c.Blocked)
	}
}
