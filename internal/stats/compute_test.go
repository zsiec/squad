package stats

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func seedItem(t *testing.T, db *sql.DB, id, status, priority, area string) {
	_, err := db.Exec(`INSERT INTO items (repo_id, item_id, title, type, priority, area,
		status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES ('repo-1', ?, 't', 'feat', ?, ?, ?, '', '', 0, 0, 0, '', ?)`,
		id, priority, area, status, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
}

func seedClaim(t *testing.T, db *sql.DB, itemID, agentID string, ts int64) {
	_, err := db.Exec(`INSERT INTO claims (item_id, repo_id, agent_id, claimed_at,
		last_touch, intent, long) VALUES (?, 'repo-1', ?, ?, ?, '', 0)`,
		itemID, agentID, ts, ts)
	if err != nil {
		t.Fatal(err)
	}
}

func seedClaimHistory(t *testing.T, db *sql.DB, itemID, agentID string, claimed, released int64, outcome string) {
	_, err := db.Exec(`INSERT INTO claim_history (repo_id, item_id, agent_id,
		claimed_at, released_at, outcome) VALUES ('repo-1', ?, ?, ?, ?, ?)`,
		itemID, agentID, claimed, released, outcome)
	if err != nil {
		t.Fatal(err)
	}
}

func TestComputeItemsCounts(t *testing.T) {
	db := openTestDB(t)
	seedItem(t, db, "BUG-001", "open", "P1", "chat")
	seedItem(t, db, "BUG-002", "open", "P2", "chat")
	seedItem(t, db, "BUG-003", "blocked", "P0", "claims")
	seedItem(t, db, "BUG-004", "done", "P2", "server")
	seedClaim(t, db, "BUG-001", "agent-a", time.Now().Unix())

	snap, err := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: time.Unix(2_000_000_000, 0), Window: 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Items.Total != 4 ||
		snap.Items.Open != 1 || snap.Items.Claimed != 1 ||
		snap.Items.Blocked != 1 || snap.Items.Done != 1 {
		t.Errorf("counts: %+v", snap.Items)
	}
}

func TestComputeClaimDurationPercentiles(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	since := now.Add(-24 * time.Hour).Unix()
	for i, d := range []int64{100, 200, 300, 400, 500} {
		seedClaimHistory(t, db, "BUG-1", "agent-a",
			since+int64(i)*60, since+int64(i)*60+d, "done")
	}
	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if snap.Claims.CompletedInWindow != 5 {
		t.Fatalf("completed: %d", snap.Claims.CompletedInWindow)
	}
	if snap.Claims.DurationSeconds.P50 == nil || *snap.Claims.DurationSeconds.P50 != 300 {
		t.Errorf("p50: %v", snap.Claims.DurationSeconds.P50)
	}
}
