package analyze

import (
	"testing"

	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
)

func TestRun_FiveItemsThreeStreams(t *testing.T) {
	a := Run(
		epics.Epic{Name: "auth-login-redirect", Spec: "auth-rework"},
		[]items.Item{
			{ID: "FEAT-001", ConflictsWith: []string{"internal/auth/login.go"}},
			{ID: "FEAT-002", ConflictsWith: []string{"internal/auth/login.go"}},
			{ID: "FEAT-003", ConflictsWith: []string{"internal/auth/session.go"}},
			{ID: "FEAT-004", ConflictsWith: []string{"internal/auth/session.go"}},
			{ID: "FEAT-005", ConflictsWith: []string{"docs/auth.md"}},
		})
	if a.Epic != "auth-login-redirect" || len(a.Streams) != 3 || a.ParallelismFactor < 1 {
		t.Errorf("epic=%q streams=%d factor=%f",
			a.Epic, len(a.Streams), a.ParallelismFactor)
	}
}

func TestRun_OverlappingConflictsCollapseStreams(t *testing.T) {
	a := Run(epics.Epic{}, []items.Item{
		{ID: "A", ConflictsWith: []string{"a.go", "shared.go"}},
		{ID: "B", ConflictsWith: []string{"shared.go", "b.go"}},
		{ID: "C", ConflictsWith: []string{"b.go"}},
		{ID: "D", ConflictsWith: []string{"d.go"}},
	})
	if len(a.Streams) != 2 {
		t.Errorf("streams=%d want 2 (ABC merged, D alone)", len(a.Streams))
	}
}

func TestRun_DepsGraphFlagsCycle(t *testing.T) {
	a := Run(epics.Epic{}, []items.Item{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"A"}},
	})
	if !a.HasCycle {
		t.Error("expected HasCycle true")
	}
}

func TestRun_ParallelismFactorZeroItems(t *testing.T) {
	a := Run(epics.Epic{}, nil)
	if a.ParallelismFactor != 0 || len(a.Streams) != 0 {
		t.Errorf("zero items: factor=%f streams=%d", a.ParallelismFactor, len(a.Streams))
	}
}
