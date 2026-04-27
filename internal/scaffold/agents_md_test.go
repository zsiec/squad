package scaffold

import (
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/specs"
)

func agentsMdFixture() AgentsMdData {
	return AgentsMdData{
		Ready: []items.Item{
			{ID: "BUG-101", Title: "claim race on concurrent worktree provision", Priority: "P1"},
			{ID: "FEAT-201", Title: "stats by capability", Priority: "P2"},
		},
		InFlight: []InFlightRow{
			{ItemID: "BUG-202", Title: "doctor missing learning emit", ClaimantID: "agent-bb", Intent: "wire propose pipeline"},
		},
		Done: []items.Item{
			{ID: "BUG-099", Title: "intake errors surface as -32603", Priority: "P3"},
			{ID: "CHORE-014", Title: "default_worktree_per_claim true in scaffold", Priority: "P2"},
		},
		Specs: []specs.Spec{
			{Name: "agent-team-management-surface", Title: "Agent-team management surface"},
		},
		Epics: []epics.Epic{
			{Name: "coordination-defaults", Spec: "agent-team-management-surface", Status: "open"},
			{Name: "documentation-contract", Spec: "agent-team-management-surface", Status: "done"},
		},
	}
}

func TestRenderAgentsMd_Banner(t *testing.T) {
	out := RenderAgentsMd(agentsMdFixture())
	if !strings.HasPrefix(strings.TrimLeft(out, " \t\n"), "<!--") {
		t.Errorf("expected leading HTML-comment banner; got prefix:\n%.200s", out)
	}
	for _, want := range []string{"do not edit by hand", "squad scaffold agents-md"} {
		if !strings.Contains(out, want) {
			t.Errorf("banner missing %q\n---\n%.300s", want, out)
		}
	}
}

func TestRenderAgentsMd_AllSectionsPresent(t *testing.T) {
	out := RenderAgentsMd(agentsMdFixture())
	for _, want := range []string{
		"## Ready",
		"BUG-101", "P1", "claim race on concurrent worktree provision",
		"## In flight",
		"BUG-202", "agent-bb", "wire propose pipeline",
		"## Recently done",
		"BUG-099", "CHORE-014",
		"## Specs",
		".squad/specs/agent-team-management-surface.md",
		"## Epics",
		".squad/epics/coordination-defaults.md",
		".squad/epics/documentation-contract.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("AGENTS.md missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderAgentsMd_Idempotent(t *testing.T) {
	a1 := RenderAgentsMd(agentsMdFixture())
	a2 := RenderAgentsMd(agentsMdFixture())
	if a1 != a2 {
		t.Fatal("RenderAgentsMd produced non-byte-identical output across runs")
	}
}

// TestRenderAgentsMd_RecentlyDoneIncludesSummary pins the FEAT-049 AC
// shape (id, title, summary) for the Recently-done section. The summary
// is the close note from `squad done --summary "..."` (recorded as a
// chat message body and looked up by the cobra wrapper).
func TestRenderAgentsMd_RecentlyDoneIncludesSummary(t *testing.T) {
	d := agentsMdFixture()
	d.Summaries = map[string]string{
		"BUG-099":   "mapped intake errors at the MCP layer",
		"CHORE-014": "flipped default_worktree_per_claim to true",
	}
	out := RenderAgentsMd(d)
	for _, want := range []string{
		"BUG-099",
		"intake errors surface as -32603",
		"mapped intake errors at the MCP layer",
		"CHORE-014",
		"default_worktree_per_claim true in scaffold",
		"flipped default_worktree_per_claim to true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Recently-done render missing %q\n---\n%s", want, out)
		}
	}
}

// TestRenderAgentsMd_RecentlyDoneFallbackForMissingSummary covers the
// case where an item closed without a --summary: the line still renders
// (drop-the-row would lose audit trail) with a clearly-marked
// placeholder so a reader knows the field was empty, not forgotten.
func TestRenderAgentsMd_RecentlyDoneFallbackForMissingSummary(t *testing.T) {
	d := agentsMdFixture()
	d.Summaries = map[string]string{} // empty — no summary recorded for either done item
	out := RenderAgentsMd(d)
	if !strings.Contains(out, "BUG-099") || !strings.Contains(out, "CHORE-014") {
		t.Fatalf("done items must still render when summaries map is empty:\n%s", out)
	}
	if !strings.Contains(out, "_(no summary)_") {
		t.Errorf("missing-summary fallback marker absent — readers cannot tell empty from absent\n---\n%s", out)
	}
}

func TestRenderAgentsMd_EmptyLedgerStillRenders(t *testing.T) {
	out := RenderAgentsMd(AgentsMdData{})
	for _, want := range []string{
		"do not edit by hand",
		"## Ready",
		"_No ready items._",
		"## In flight",
		"_No active claims._",
		"## Recently done",
		"_No items closed._",
		"## Specs",
		"_No active specs._",
		"## Epics",
		"_No active epics._",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty-ledger render missing %q\n---\n%s", want, out)
		}
	}
}
