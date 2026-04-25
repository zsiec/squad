package items

import (
	"testing"
	"time"
)

func mustWalkReady(t *testing.T) WalkResult {
	t.Helper()
	w, err := Walk("testdata/ready")
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestReady_OrdersByPriorityThenSmallestEstimate(t *testing.T) {
	w := mustWalkReady(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	got := Ready(w, now)
	if len(got) == 0 {
		t.Fatal("expected non-empty ready list")
	}
	if got[0].ID != "FEAT-006" {
		t.Fatalf("got[0]=%s want FEAT-006", got[0].ID)
	}
	if got[1].ID != "FEAT-004" {
		t.Fatalf("got[1]=%s want FEAT-004", got[1].ID)
	}
}

func TestReady_FiltersOutItemsBlockedByOpen(t *testing.T) {
	w := mustWalkReady(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	got := Ready(w, now)
	for _, it := range got {
		if it.ID == "FEAT-002" {
			t.Fatal("FEAT-002 should be filtered out (blocked by open BUG-005)")
		}
	}
}

func TestReady_FiltersOutFutureNotBefore(t *testing.T) {
	w := mustWalkReady(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	got := Ready(w, now)
	for _, it := range got {
		if it.ID == "FEAT-003" {
			t.Fatal("FEAT-003 should be filtered out (not-before 2099)")
		}
	}
}

func TestReady_TiebreakerSmallestEstimate(t *testing.T) {
	w := mustWalkReady(t)
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	got := Ready(w, now)
	var p2 []string
	for _, it := range got {
		if it.Priority == "P2" {
			p2 = append(p2, it.ID)
		}
	}
	if len(p2) < 2 || p2[0] != "BUG-005" {
		t.Fatalf("p2 order=%v want BUG-005 first", p2)
	}
}
