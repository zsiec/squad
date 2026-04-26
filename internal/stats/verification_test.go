package stats

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func ensureAttestationsTable(t *testing.T, db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS attestations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		repo_id TEXT NOT NULL, item_id TEXT NOT NULL,
		kind TEXT NOT NULL, command TEXT, exit_code INTEGER,
		output_hash TEXT, output_path TEXT,
		created_at INTEGER NOT NULL, agent_id TEXT,
		review_disagreements INTEGER DEFAULT 0)`)
	if err != nil {
		t.Fatal(err)
	}
}

func seedAttestation(t *testing.T, db *sql.DB, itemID, kind string, exit int, when int64, disagreements int) {
	_, err := db.Exec(`INSERT INTO attestations
		(repo_id, item_id, kind, command, exit_code, output_hash, output_path,
		 created_at, agent_id, review_disagreements)
		VALUES ('repo-1', ?, ?, '', ?, ?, '', ?, 'agent-a', ?)`,
		itemID, kind, exit, fmt.Sprintf("%s-%s-%d", itemID, kind, when), when, disagreements)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerificationRateFullEvidence(t *testing.T) {
	db := openTestDB(t)
	ensureAttestationsTable(t, db)
	now := time.Unix(2_000_000_000, 0)
	since := now.Add(-24*time.Hour).Unix() + 100
	for i, id := range []string{"BUG-1", "BUG-2", "BUG-3"} {
		seedClaimHistory(t, db, id, "agent-a",
			since+int64(i)*100, since+int64(i)*100+50, "done")
	}
	for _, id := range []string{"BUG-1", "BUG-2"} {
		seedAttestation(t, db, id, "test", 0, since+10, 0)
		seedAttestation(t, db, id, "review", 0, since+20, 0)
	}
	seedAttestation(t, db, "BUG-3", "test", 0, since+50, 0) // no review → not full

	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if snap.Verification.DonesTotal != 3 || snap.Verification.DonesWithFullEvidence != 2 {
		t.Fatalf("dones: %+v", snap.Verification)
	}
	if r := snap.Verification.Rate; r == nil || *r < 0.66 || *r > 0.67 {
		t.Errorf("rate: %v", r)
	}
}

func TestReviewerDisagreementRate(t *testing.T) {
	db := openTestDB(t)
	ensureAttestationsTable(t, db)
	now := time.Unix(2_000_000_000, 0)
	since := now.Add(-24*time.Hour).Unix() + 100
	for i := 0; i < 4; i++ {
		seedAttestation(t, db, "BUG-X", "review", 0, since+int64(i)*10, 0)
	}
	seedAttestation(t, db, "BUG-X", "review", 0, since+50, 2)
	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if snap.Verification.ReviewsTotal != 5 || snap.Verification.ReviewsWithDisagreement != 1 {
		t.Errorf("reviews: %+v", snap.Verification)
	}
	if r := snap.Verification.ReviewerDisagreementRate; r == nil || *r != 0.2 {
		t.Errorf("rate: %v", r)
	}
}
