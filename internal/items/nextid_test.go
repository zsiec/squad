package items

import "testing"

func TestNextID_PicksMaxPlusOneAcrossActiveAndDone(t *testing.T) {
	got, err := Walk("testdata")
	if err != nil {
		t.Fatal(err)
	}
	next, err := NextID("BUG", got)
	if err != nil {
		t.Fatal(err)
	}
	if next != "BUG-002" {
		t.Fatalf("got %s want BUG-002", next)
	}
	next, err = NextID("FEAT", got)
	if err != nil {
		t.Fatal(err)
	}
	if next != "FEAT-003" {
		t.Fatalf("got %s want FEAT-003", next)
	}
	next, err = NextID("TASK", got)
	if err != nil {
		t.Fatal(err)
	}
	if next != "TASK-001" {
		t.Fatalf("got %s want TASK-001", next)
	}
}
