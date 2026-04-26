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

func TestReady_GatesByDependsOn(t *testing.T) {
	w, err := Walk("testdata/ready")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	for _, it := range Ready(w, now) {
		if it.ID == "FEAT-201" {
			t.Fatal("FEAT-201 should be filtered (dep FEAT-200 not done)")
		}
	}
}

func TestReady_DependsOnSatisfiedSurfaces(t *testing.T) {
	w, err := Walk("testdata/ready")
	if err != nil {
		t.Fatal(err)
	}
	for i, it := range w.Active {
		if it.ID == "FEAT-200" {
			w.Active[i].Status = "done"
		}
	}
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	found := false
	for _, it := range Ready(w, now) {
		if it.ID == "FEAT-201" {
			found = true
		}
	}
	if !found {
		t.Fatal("FEAT-201 should surface once dep is done")
	}
}

func TestReady_DependsOnUnknownIDIsUnsatisfied(t *testing.T) {
	w := WalkResult{Active: []Item{{
		ID: "FEAT-300", Title: "x", Status: "open", Priority: "P1",
		DependsOn: []string{"FEAT-DOES-NOT-EXIST"},
	}}}
	for _, it := range Ready(w, time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)) {
		if it.ID == "FEAT-300" {
			t.Fatal("unknown dep must gate ready surfacing")
		}
	}
}

func TestReady_ExcludesCaptured(t *testing.T) {
	w := WalkResult{Active: []Item{
		{ID: "FEAT-001", Status: "captured", Priority: "P0"},
		{ID: "FEAT-002", Status: "open", Priority: "P1"},
	}}
	out := Ready(w, time.Now())
	if len(out) != 1 || out[0].ID != "FEAT-002" {
		t.Fatalf("want only FEAT-002 ready; got %+v", out)
	}
}

func TestReady_ExcludesUnknownStatus(t *testing.T) {
	w := WalkResult{Active: []Item{
		{ID: "FEAT-001", Status: "weird", Priority: "P0"},
		{ID: "FEAT-002", Status: "open", Priority: "P1"},
	}}
	out := Ready(w, time.Now())
	if len(out) != 1 || out[0].ID != "FEAT-002" {
		t.Fatalf("unknown status should be excluded; got %+v", out)
	}
}
