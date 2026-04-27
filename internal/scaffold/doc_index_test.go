package scaffold

import (
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/specs"
)

// fixture builds a small but representative ledger snapshot: one spec
// with two epics (one active, one done) plus an orphan epic, items
// linked to each epic by name. Mirrors what RenderDocIndex sees from
// real Walk callers.
func docIndexFixture() (specsList []specs.Spec, epicsList []epics.Epic, itemsList []items.Item) {
	specsList = []specs.Spec{{
		Name:  "agent-team-management-surface",
		Title: "Agent-team management surface",
	}}
	epicsList = []epics.Epic{
		{Name: "coordination-defaults", Spec: "agent-team-management-surface", Status: "open"},
		{Name: "documentation-contract", Spec: "agent-team-management-surface", Status: "done"},
		{Name: "orphan", Spec: "", Status: "open"},
	}
	itemsList = []items.Item{
		{ID: "FEAT-1", Epic: "coordination-defaults", Status: "open"},
		{ID: "FEAT-2", Epic: "coordination-defaults", Status: "done"},
		{ID: "CHORE-1", Epic: "coordination-defaults", Status: "open"},
		{ID: "FEAT-3", Epic: "documentation-contract", Status: "done"},
		{ID: "FEAT-4", Epic: "orphan", Status: "open"},
	}
	return
}

func TestRenderDocIndex_SpecsLinkAndStatus(t *testing.T) {
	s, e, it := docIndexFixture()
	specsMD, _ := RenderDocIndex(s, e, it)
	for _, want := range []string{
		"Agent-team management surface",
		".squad/specs/agent-team-management-surface.md",
		"active",
	} {
		if !strings.Contains(specsMD, want) {
			t.Errorf("specs.md missing %q\n---\n%s", want, specsMD)
		}
	}
}

func TestRenderDocIndex_EpicsLinkStatusAndItemCount(t *testing.T) {
	s, e, it := docIndexFixture()
	_, epicsMD := RenderDocIndex(s, e, it)
	for _, want := range []string{
		".squad/epics/coordination-defaults.md",
		".squad/epics/documentation-contract.md",
		".squad/epics/orphan.md",
		"3 items",
		"1 item",
		"active",
		"done",
	} {
		if !strings.Contains(epicsMD, want) {
			t.Errorf("epics.md missing %q\n---\n%s", want, epicsMD)
		}
	}
}

// TestRenderDocIndex_Idempotent verifies running render twice with no
// ledger changes yields byte-identical output, per the AC's Notes
// guidance.
func TestRenderDocIndex_Idempotent(t *testing.T) {
	s, e, it := docIndexFixture()
	a1, b1 := RenderDocIndex(s, e, it)
	a2, b2 := RenderDocIndex(s, e, it)
	if a1 != a2 || b1 != b2 {
		t.Fatalf("RenderDocIndex not byte-identical across runs")
	}
}

// TestRenderDocIndex_AllEpicsDoneMarksSpecDone covers the spec status
// derivation: a spec whose every child epic is done renders as "done"
// in the spec index. Without a Status field on Spec, this aggregation
// is the only meaningful answer.
func TestRenderDocIndex_AllEpicsDoneMarksSpecDone(t *testing.T) {
	specsList := []specs.Spec{{Name: "small-spec", Title: "Small spec"}}
	epicsList := []epics.Epic{
		{Name: "ep1", Spec: "small-spec", Status: "done"},
		{Name: "ep2", Spec: "small-spec", Status: "done"},
	}
	specsMD, _ := RenderDocIndex(specsList, epicsList, nil)
	if !strings.Contains(specsMD, "— done\n") {
		t.Errorf("spec with all-done epics should render with `— done` suffix; got:\n%s", specsMD)
	}
	if strings.Contains(specsMD, "— active\n") {
		t.Errorf("spec with all-done epics should NOT render with `— active` suffix; got:\n%s", specsMD)
	}
}
