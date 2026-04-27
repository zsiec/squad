package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
)

// runScaffoldAgentsMd executes the agents-md cobra subcommand against
// the current working directory with the supplied flags. Returns the
// command-side error so tests can pin success/drift exits.
func runScaffoldAgentsMd(t *testing.T, args ...string) error {
	t.Helper()
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	full := append([]string{"scaffold", "agents-md"}, args...)
	root.SetArgs(full)
	root.SilenceErrors = true
	root.SilenceUsage = true
	for _, c := range root.Commands() {
		silenceErrUsage(c)
	}
	return root.Execute()
}

func silenceErrUsage(c *cobra.Command) {
	c.SilenceErrors = true
	c.SilenceUsage = true
	for _, sub := range c.Commands() {
		silenceErrUsage(sub)
	}
}

// TestScaffoldAgentsMd_CheckPassesWhenInSync writes a fresh AGENTS.md,
// then re-runs with --check; the file matches the generator output and
// the check exits 0.
func TestScaffoldAgentsMd_CheckPassesWhenInSync(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	if err := runScaffoldAgentsMd(t); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if err := runScaffoldAgentsMd(t, "--check"); err != nil {
		t.Fatalf("--check on freshly-generated file should pass; got %v", err)
	}
}

// TestScaffoldAgentsMd_CheckFailsOnDrift hand-edits AGENTS.md after
// generation; --check must surface the drift via a non-zero exit so
// the pre-commit hook can refuse the commit.
func TestScaffoldAgentsMd_CheckFailsOnDrift(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	if err := runScaffoldAgentsMd(t); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	path := filepath.Join(repo, "AGENTS.md")
	body, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(body, []byte("\nhand edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runScaffoldAgentsMd(t, "--check"); err == nil {
		t.Fatalf("--check on hand-edited AGENTS.md should fail; got nil")
	}
}

// TestScaffoldAgentsMd_CheckDoesNotWrite verifies --check is a pure
// observation — the file is left in its drifted state for the
// operator to fix, not silently rewritten.
func TestScaffoldAgentsMd_CheckDoesNotWrite(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	if err := runScaffoldAgentsMd(t); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	path := filepath.Join(repo, "AGENTS.md")
	const drifted = "<!-- handcrafted -->\n# notes\n"
	if err := os.WriteFile(path, []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runScaffoldAgentsMd(t, "--check"); err == nil {
		t.Fatalf("--check should fail on drift")
	}
	body, _ := os.ReadFile(path)
	if string(body) != drifted {
		t.Errorf("--check wrote to AGENTS.md; expected file untouched")
	}
}

// TestPickDone_SortsByUpdatedDESC pins the recency contract — items.Walk
// returns done in os.ReadDir (alphabetic) order, so without explicit
// sort the "last 10" section would surface old items, not recent ones.
func TestPickDone_SortsByUpdatedDESC(t *testing.T) {
	in := []items.Item{
		{ID: "BUG-A", Title: "old", Updated: "2026-01-01"},
		{ID: "BUG-B", Title: "newest", Updated: "2026-04-30"},
		{ID: "BUG-C", Title: "middle", Updated: "2026-03-15"},
	}
	got := pickDone(in, 10)
	wantIDs := []string{"BUG-B", "BUG-C", "BUG-A"}
	for i, w := range wantIDs {
		if got[i].ID != w {
			t.Errorf("pickDone[%d] = %s; want %s (updated DESC order)", i, got[i].ID, w)
		}
	}
}

// TestPickDone_CapsAtN pins the per-section item cap.
func TestPickDone_CapsAtN(t *testing.T) {
	in := make([]items.Item, 15)
	for i := range in {
		in[i] = items.Item{ID: "BUG-X", Updated: "2026-04-30"}
	}
	if got := pickDone(in, 10); len(got) != 10 {
		t.Errorf("pickDone returned %d; want capped at 10", len(got))
	}
}

// TestLoadInFlightRows_JoinsClaimsWithItemTitles exercises the real
// cobra-path helper against a seeded DB row + an in-memory item list,
// closing the AC#5 gap (test against fixture DB).
func TestLoadInFlightRows_JoinsClaimsWithItemTitles(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	env := newTestEnv(t)
	if _, err := env.DB.Exec(
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, 1, 1, 'wire the pipeline', 0)`,
		env.RepoID, "BUG-505", env.AgentID,
	); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	active := []items.Item{
		{ID: "BUG-505", Title: "doctor missing learning emit"},
	}

	rows, err := loadInFlightRows(context.Background(), env.DB, env.RepoID, active)
	if err != nil {
		t.Fatalf("loadInFlightRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d (%+v)", len(rows), rows)
	}
	r := rows[0]
	if r.ItemID != "BUG-505" || r.Title != "doctor missing learning emit" ||
		r.ClaimantID != env.AgentID || r.Intent != "wire the pipeline" {
		t.Errorf("row mismatch: %+v", r)
	}
}

// TestLoadInFlightRows_OrphanClaimGetsPlaceholderTitle pins that a claim
// pointing at an item not on disk is rendered with a marker title
// rather than dropped silently — operator visibility over data loss.
func TestLoadInFlightRows_OrphanClaimGetsPlaceholderTitle(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	env := newTestEnv(t)
	if _, err := env.DB.Exec(
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, 1, 1, '', 0)`,
		env.RepoID, "ORPHAN-999", env.AgentID,
	); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	rows, err := loadInFlightRows(context.Background(), env.DB, env.RepoID, nil)
	if err != nil {
		t.Fatalf("loadInFlightRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Title == "" || rows[0].Title == "ORPHAN-999" {
		t.Errorf("orphan should get placeholder Title; got %q", rows[0].Title)
	}
}
