package stats

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// seedItemWithCapability inserts an item with a literal JSON-array
// requires_capability value. Mirrors how items.persistUpsert writes
// the column — `[]` for untagged, otherwise a JSON list.
func seedItemWithCapability(t *testing.T, db *sql.DB, id, status, capability string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO items (repo_id, item_id, title, type, priority, area,
			status, estimate, risk, ac_total, ac_checked, archived, path, updated_at, requires_capability)
		VALUES ('repo-1', ?, 't', 'feat', 'P2', 'general', ?, '', '', 0, 0, 0, '', ?, ?)`,
		id, status, time.Now().Unix(), capability,
	); err != nil {
		t.Fatalf("seed item %s: %v", id, err)
	}
}

func TestByCapability_SingleMultiUntaggedAndWindow(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	inWindow := now.Add(-3 * time.Hour).Unix()
	outOfWindow := now.Add(-48 * time.Hour).Unix()

	// Single-tag, in window.
	seedItemWithCapability(t, db, "BUG-1", "done", `["go"]`)
	seedClaimHistory(t, db, "BUG-1", "agent-a", inWindow-100, inWindow, "done")

	// Multi-tag, in window — increments both buckets once each.
	seedItemWithCapability(t, db, "BUG-2", "done", `["go","sql"]`)
	seedClaimHistory(t, db, "BUG-2", "agent-a", inWindow-100, inWindow, "done")

	// Untagged ([]), in window — counted under (untagged).
	seedItemWithCapability(t, db, "BUG-3", "done", `[]`)
	seedClaimHistory(t, db, "BUG-3", "agent-a", inWindow-100, inWindow, "done")

	// Out-of-window — must NOT count.
	seedItemWithCapability(t, db, "BUG-4", "done", `["frontend"]`)
	seedClaimHistory(t, db, "BUG-4", "agent-a", outOfWindow-100, outOfWindow, "done")

	// In-progress (released, not done) — must NOT count.
	seedItemWithCapability(t, db, "BUG-5", "open", `["go"]`)
	seedClaimHistory(t, db, "BUG-5", "agent-a", inWindow-100, inWindow, "released")

	snap, err := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	got := map[string]int64{}
	for _, r := range snap.ByCapability {
		got[r.Capability] = r.DoneCount
	}
	want := map[string]int64{
		"go":         2, // BUG-1 + BUG-2
		"sql":        1, // BUG-2
		"(untagged)": 1, // BUG-3
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("ByCapability[%q] = %d; want %d (got=%v)", k, got[k], w, got)
		}
	}
	if _, has := got["frontend"]; has {
		t.Errorf("frontend should be excluded by window; got %d", got["frontend"])
	}
}

func TestByAgentSortAndCap(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	base := now.Add(-12 * time.Hour).Unix()
	for i := 0; i < 60; i++ {
		seedClaimHistory(t, db, "BUG-X",
			"agent-"+string(rune('a'+i%30)),
			base+int64(i), base+int64(i)+100, "done")
	}
	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if len(snap.ByAgent) > 51 {
		t.Errorf("by_agent should be capped at 50+_other; got %d", len(snap.ByAgent))
	}
	prev := int64(1 << 62)
	for _, a := range snap.ByAgent {
		if a.AgentID == "_other" {
			continue
		}
		if a.ClaimsCompleted > prev {
			t.Errorf("by_agent not sorted desc: %d > %d", a.ClaimsCompleted, prev)
		}
		prev = a.ClaimsCompleted
	}
}

func TestByAgentSpillRollsUpReleaseCount(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	base := now.Add(-12 * time.Hour).Unix()

	// 60 distinct agents, each with 1 done + 1 released. 50 fit; 10 spill.
	for i := 0; i < 60; i++ {
		agent := "agent-" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		seedClaimHistory(t, db, "BUG-X", agent, base+int64(2*i), base+int64(2*i)+10, "done")
		seedClaimHistory(t, db, "BUG-X", agent, base+int64(2*i+1000), base+int64(2*i+1010), "released")
	}

	snap, err := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	var spill *AgentRow
	for i := range snap.ByAgent {
		if snap.ByAgent[i].AgentID == "_other" {
			spill = &snap.ByAgent[i]
			break
		}
	}
	if spill == nil {
		t.Fatal("expected _other spill row, got none")
	}
	if spill.ClaimsCompleted != 10 || spill.ReleaseCount != 10 {
		t.Errorf("spill: done=%d release=%d, want 10/10", spill.ClaimsCompleted, spill.ReleaseCount)
	}
	if spill.Ratio == nil || *spill.Ratio != 1.0 {
		t.Errorf("spill ratio=%v want 1.0", spill.Ratio)
	}
}

func TestByAgentDoneReleaseRatio(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	base := now.Add(-12 * time.Hour).Unix()

	// agent-a: 6 done, 2 released → ratio 3.0
	for i := 0; i < 6; i++ {
		seedClaimHistory(t, db, "BUG-A", "agent-a", base+int64(i), base+int64(i)+10, "done")
	}
	for i := 0; i < 2; i++ {
		seedClaimHistory(t, db, "BUG-A", "agent-a", base+int64(100+i), base+int64(110+i), "released")
	}
	// agent-b: 4 done, 0 released → ratio nil (rendered as "-" by CLI)
	for i := 0; i < 4; i++ {
		seedClaimHistory(t, db, "BUG-B", "agent-b", base+int64(200+i), base+int64(210+i), "done")
	}
	// agent-c: 1 done, 4 released → ratio 0.25
	seedClaimHistory(t, db, "BUG-C", "agent-c", base+int64(300), base+int64(310), "done")
	for i := 0; i < 4; i++ {
		seedClaimHistory(t, db, "BUG-C", "agent-c", base+int64(311+i), base+int64(320+i), "released")
	}

	snap, err := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	byID := map[string]AgentRow{}
	for _, r := range snap.ByAgent {
		byID[r.AgentID] = r
	}

	a := byID["agent-a"]
	if a.ClaimsCompleted != 6 || a.ReleaseCount != 2 {
		t.Errorf("agent-a: done=%d releases=%d, want 6/2", a.ClaimsCompleted, a.ReleaseCount)
	}
	if a.Ratio == nil || *a.Ratio != 3.0 {
		t.Errorf("agent-a ratio=%v want 3.0", a.Ratio)
	}

	b := byID["agent-b"]
	if b.ClaimsCompleted != 4 || b.ReleaseCount != 0 {
		t.Errorf("agent-b: done=%d releases=%d, want 4/0", b.ClaimsCompleted, b.ReleaseCount)
	}
	if b.Ratio != nil {
		t.Errorf("agent-b ratio=%v want nil (zero releases)", *b.Ratio)
	}

	c := byID["agent-c"]
	if c.ClaimsCompleted != 1 || c.ReleaseCount != 4 {
		t.Errorf("agent-c: done=%d releases=%d, want 1/4", c.ClaimsCompleted, c.ReleaseCount)
	}
	if c.Ratio == nil || *c.Ratio != 0.25 {
		t.Errorf("agent-c ratio=%v want 0.25", c.Ratio)
	}
}

func TestSeriesBucketsByDay(t *testing.T) {
	db := openTestDB(t)
	ensureAttestationsTable(t, db)
	day1 := int64(1_745_424_000)
	day2 := day1 + 86400
	for _, id := range []string{"BUG-1", "BUG-2"} {
		seedClaimHistory(t, db, id, "agent-a", day1+10, day1+100, "done")
	}
	seedAttestation(t, db, "BUG-1", "test", 0, day1+50, 0)
	seedAttestation(t, db, "BUG-1", "review", 0, day1+60, 0)
	for _, id := range []string{"BUG-3", "BUG-4"} {
		seedClaimHistory(t, db, id, "agent-a", day2+10, day2+100, "done")
	}
	seedAttestation(t, db, "BUG-3", "test", 0, day2+50, 0)
	seedAttestation(t, db, "BUG-3", "review", 0, day2+60, 0)

	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: time.Unix(day2+5*3600, 0), Window: 72 * time.Hour,
	})
	if len(snap.Series.VerificationRateDaily) != 2 {
		t.Fatalf("verify series: %d points", len(snap.Series.VerificationRateDaily))
	}
	if snap.Series.VerificationRateDaily[0].Count != 2 ||
		snap.Series.VerificationRateDaily[0].Rate != 0.5 {
		t.Errorf("day1: %+v", snap.Series.VerificationRateDaily[0])
	}
}
