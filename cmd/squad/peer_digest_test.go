package main

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// peerDigestFixture seeds an empty fixture DB; the per-test setup
// inserts whatever rows the case needs. Returns the wired (*sql.DB,
// repoID, time anchor) so output is deterministic across runs.
func peerDigestFixture(t *testing.T) (*sql.DB, string, time.Time) {
	t.Helper()
	f := newChatFixture(t)
	now := time.Date(2026, 4, 28, 22, 0, 0, 0, time.UTC)
	return f.db, f.repoID, now
}

func insertPeer(t *testing.T, db *sql.DB, repoID, agentID, displayName, itemID, area string, lastTouch int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, '/tmp/wt', 1, 0, 0, 'active')
		ON CONFLICT(id) DO NOTHING
	`, agentID, repoID, displayName); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO claims (item_id, repo_id, agent_id, claimed_at, last_touch, intent, long, worktree)
		VALUES (?, ?, ?, ?, ?, '', 0, '')
	`, itemID, repoID, agentID, lastTouch, lastTouch); err != nil {
		t.Fatalf("insert claim: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO items (repo_id, item_id, title, type, priority, area, status,
		                   estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES (?, ?, ?, 'feature', 'P2', ?, 'open', '', '', 0, 0, 0, '', 0)
		ON CONFLICT(repo_id, item_id) DO NOTHING
	`, repoID, itemID, itemID+" title", area); err != nil {
		t.Fatalf("insert item: %v", err)
	}
}

func insertPeerMessage(t *testing.T, db *sql.DB, repoID, agentID, itemID, kind, body string, ts int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, ?, ?, ?, '', 'normal')
	`, repoID, ts, agentID, itemID, kind, body); err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func TestPeerDigest_ZeroPeers(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "peers: none active.") {
		t.Errorf("zero-peer digest should print 'peers: none active.'; got %q", got)
	}
}

func TestPeerDigest_OnePeerNoOverlap(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	insertPeer(t, db, repoID, "agent-other", "Other", "BUG-101", "internal/server", now.Add(-3*time.Minute).Unix())
	insertPeerMessage(t, db, repoID, "agent-other", "BUG-101", "thinking", "trying option B", now.Add(-3*time.Minute).Unix())

	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "internal/items", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, "overlaps with") {
		t.Errorf("non-overlapping area should not nudge; got %q", got)
	}
	if !strings.Contains(got, "@Other on BUG-101") {
		t.Errorf("digest missing peer line; got %q", got)
	}
	if !strings.Contains(got, "area=internal/server") {
		t.Errorf("digest missing area; got %q", got)
	}
	if !strings.Contains(got, "thinking: trying option B") {
		t.Errorf("digest missing excerpt; got %q", got)
	}
}

func TestPeerDigest_OnePeerWithAreaOverlap(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	insertPeer(t, db, repoID, "agent-other", "Other", "BUG-101", "internal/server", now.Add(-1*time.Minute).Unix())
	insertPeerMessage(t, db, repoID, "agent-other", "BUG-101", "fyi", "touching shared.go", now.Add(-1*time.Minute).Unix())

	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "internal/server", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "overlaps with @Other on BUG-101 (area=internal/server)") {
		t.Errorf("overlap nudge missing or malformed; got %q", got)
	}
	if !strings.Contains(got, "squad ask @Other") {
		t.Errorf("nudge should suggest 'squad ask @Other'; got %q", got)
	}
}

func TestPeerDigest_SevenPeersTruncatedToSixPlusOne(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	for i := 0; i < 7; i++ {
		ts := now.Add(time.Duration(-i) * time.Minute).Unix()
		insertPeer(t, db, repoID,
			"agent-p"+string(rune('a'+i)),
			"Peer"+string(rune('A'+i)),
			"BUG-"+string(rune('A'+i)),
			"area-"+string(rune('a'+i)),
			ts)
	}
	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "… (+1 more)") {
		t.Errorf("seven peers should truncate to 6 with '+1 more'; got %q", got)
	}
	// Six rows + the truncation line + trailing blank line. Easier to
	// assert the cap-row-presence than every row.
	rows := strings.Count(got, "\n  @")
	if rows != peerDigestCap {
		t.Errorf("expected exactly %d rendered peer rows; got %d in %q", peerDigestCap, rows, got)
	}
}
