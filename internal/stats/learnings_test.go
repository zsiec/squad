package stats

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func ensureLearningsIndex(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS learnings_index (
		repo_id TEXT NOT NULL, area TEXT NOT NULL,
		learning_path TEXT NOT NULL, approved_at INTEGER NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRepeatMistakeRate(t *testing.T) {
	db := openTestDB(t)
	ensureLearningsIndex(t, db)
	now := time.Unix(2_000_000_000, 0)
	since := now.Add(-24*time.Hour).Unix() + 100
	for _, id := range []string{"BUG-1", "BUG-2"} {
		_, _ = db.Exec(`INSERT INTO items (repo_id, item_id, title, type, priority, area,
			status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
			VALUES ('repo-1', ?, 't', 'bug', 'P2', 'chat', 'open', '', '', 0,0,0,'', ?)`,
			id, since+10)
	}
	_, _ = db.Exec(`INSERT INTO items (repo_id, item_id, title, type, priority, area,
		status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES ('repo-1', 'BUG-3', 't', 'bug', 'P2', 'server', 'open', '','',0,0,0,'',?)`,
		since+10)
	_, _ = db.Exec(`INSERT INTO learnings_index (repo_id, area, learning_path, approved_at)
		VALUES ('repo-1', 'chat', 'learnings/chat-pitfall.md', ?)`, since-1000)

	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if snap.Learnings.NewBugsInWindow != 3 || snap.Learnings.RepeatMistakesInWindow != 2 {
		t.Fatalf("learnings: %+v", snap.Learnings)
	}
	if r := snap.Learnings.RepeatMistakeRate; r == nil || *r < 0.66 || *r > 0.67 {
		t.Errorf("rate: %v", r)
	}
}

func TestTokensReportsUnavailableByDefault(t *testing.T) {
	db := openTestDB(t)
	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: time.Unix(2_000_000_000, 0), Window: 24 * time.Hour,
	})
	if snap.Tokens.PerItemEstimateMethod != "unavailable" || snap.Tokens.PerItemEstimateBytes != nil {
		t.Errorf("tokens: %+v", snap.Tokens)
	}
}
