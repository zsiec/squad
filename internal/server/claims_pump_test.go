package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func seedClaim(t *testing.T, db *sql.DB, repoID, itemID, agentID string, ts int64) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO claims (item_id, repo_id, agent_id, claimed_at, last_touch, intent, long) VALUES (?, ?, ?, ?, ?, '', 0)`,
		itemID, repoID, agentID, ts, ts,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func seedClaimHistory(t *testing.T, db *sql.DB, repoID, itemID, agentID string, claimedAt, releasedAt int64, outcome string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome) VALUES (?, ?, ?, ?, ?, ?)`,
		repoID, itemID, agentID, claimedAt, releasedAt, outcome,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSE_ItemChanged_Claimed(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Allow pump baseline tick to capture initial claims (none).
	time.Sleep(700 * time.Millisecond)

	// Insert a fresh claim — pump should emit item_changed{kind:claimed} on next tick.
	seedClaim(t, db, testRepoID, "BUG-100", "agent-7c4a", time.Now().Unix())

	if !waitSSEEvent(t, resp.Body, "item_changed", 3*time.Second) {
		t.Fatal("did not see item_changed event after claim insert")
	}
}

func TestSSE_ItemChanged_Released(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	time.Sleep(700 * time.Millisecond)

	// Insert a claim_history row — pump emits item_changed with the outcome.
	now := time.Now().Unix()
	seedClaimHistory(t, db, testRepoID, "BUG-100", "agent-7c4a", now-60, now, "done")

	if !waitSSEEvent(t, resp.Body, "item_changed", 3*time.Second) {
		t.Fatal("did not see item_changed event after claim_history insert")
	}
}
