package main

import (
	"context"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

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
