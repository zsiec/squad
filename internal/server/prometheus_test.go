package server

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seedDoneClaim(t *testing.T, db *sql.DB, repoID, itemID, agentID string, claimedAt, releasedAt int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, ?, ?, ?, ?, 'done')`,
		repoID, itemID, agentID, claimedAt, releasedAt); err != nil {
		t.Fatalf("seed done claim: %v", err)
	}
}

func seedAttestationRow(t *testing.T, db *sql.DB, repoID, itemID, kind string, exitCode, disagreements int, createdAt int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO attestations (repo_id, item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, review_disagreements)
		VALUES (?, ?, ?, '', ?, ?, '', ?, 'agent-a', ?)`,
		repoID, itemID, kind, exitCode,
		repoID+"-"+itemID+"-"+kind,
		createdAt, disagreements); err != nil {
		t.Fatalf("seed attestation: %v", err)
	}
}

func TestMetricsExposesAllSquadFamilies(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	var nowUnix int64
	if err := db.QueryRowContext(ctx, `SELECT CAST(strftime('%s','now') AS INTEGER)`).Scan(&nowUnix); err != nil {
		t.Fatalf("now: %v", err)
	}
	claimed := nowUnix - 3600
	released := nowUnix - 3500

	seedDoneClaim(t, db, "repo-1", "BUG-1", "agent-a", claimed, released)
	seedAttestationRow(t, db, "repo-1", "BUG-1", "test", 0, 0, released)
	seedAttestationRow(t, db, "repo-1", "BUG-1", "review", 0, 1, released)

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS learnings_index (
		repo_id TEXT NOT NULL, area TEXT NOT NULL,
		learning_path TEXT NOT NULL, approved_at INTEGER NOT NULL)`); err != nil {
		t.Fatalf("create learnings_index: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO items (repo_id, item_id, title, type, priority, area,
		status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES ('repo-1', 'BUG-2', 't', 'bug', 'P2', 'core', 'open', '', '', 0, 0, 0, '', ?)`,
		nowUnix-3600); err != nil {
		t.Fatalf("seed bug item: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO learnings_index (repo_id, area, learning_path, approved_at)
		VALUES ('repo-1', 'core', 'learnings/x.md', ?)`, nowUnix-7200); err != nil {
		t.Fatalf("seed learning: %v", err)
	}

	s := New(db, "repo-1", Config{RepoID: "repo-1"})
	defer s.Close()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("get /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		"squad_items_total",
		"squad_claim_duration_seconds",
		"squad_verification_rate",
		"squad_reviewer_disagreement_rate",
		"squad_wip_violations_attempted_total",
		"squad_repeat_mistake_rate",
		"squad_attestations_total",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing metric family %q\nbody:\n%s", want, body)
		}
	}
}
