package hygiene

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDumpStatus_Deterministic(t *testing.T) {
	db := newDB(t)
	registerAgent(t, db, "repo-test", "agent-a", 1_700_000_000)
	insertClaim(t, db, "repo-test", "BUG-1", "agent-a", 1_700_000_000, 0)
	if _, err := db.Exec(`UPDATE claims SET intent='fix it' WHERE item_id='BUG-1'`); err != nil {
		t.Fatal(err)
	}

	items := []StatusItem{
		{ID: "FEAT-2", Title: "Add a thing", Priority: "P0", Status: "ready", Estimate: "1h"},
		{ID: "FEAT-3", Title: "Add another", Priority: "P1", Status: "ready", Estimate: "2h"},
		{ID: "BUG-9", Title: "Mystery", Priority: "P2", Status: "blocked", Estimate: "?"},
	}
	clock := func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	out, err := DumpStatus(context.Background(), db, "repo-test", items, clock)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Status Board", "## In Progress", "BUG-1 (agent-a): fix it",
		"## Ready — P0", "FEAT-2  Add a thing  (1h)",
		"## Ready — P1", "FEAT-3  Add another  (2h)",
		"## Blocked", "BUG-9  Mystery",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	out2, _ := DumpStatus(context.Background(), db, "repo-test", items, clock)
	if out != out2 {
		t.Fatalf("non-deterministic output")
	}
}
