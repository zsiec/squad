package main

import (
	"context"
	"testing"
	"time"
)

func TestStandup_GathersClosedReclaimedAndOpen(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	now := time.Now().Unix()
	dayAgo := now - int64((25 * time.Hour).Seconds())
	hourAgo := now - 3600

	// Two closed-by-me events in the window.
	if _, err := f.db.Exec(`
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, 'BUG-1', ?, ?, ?, 'done'), (?, 'FEAT-2', ?, ?, ?, 'done')
	`, f.repoID, f.agentID, hourAgo-3600, hourAgo,
		f.repoID, f.agentID, hourAgo-1800, now-600); err != nil {
		t.Fatal(err)
	}

	// One older 'done' outside the window — should NOT appear.
	if _, err := f.db.Exec(`
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, 'OLD-9', ?, ?, ?, 'done')
	`, f.repoID, f.agentID, dayAgo-7200, dayAgo); err != nil {
		t.Fatal(err)
	}

	// One reclaimed in the window.
	if _, err := f.db.Exec(`
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, 'CHORE-3', ?, ?, ?, 'reclaimed')
	`, f.repoID, f.agentID, hourAgo-7200, now-300); err != nil {
		t.Fatal(err)
	}

	// One currently-open claim.
	f.insertClaim(t, "FEAT-7")
	if _, err := f.db.Exec(`UPDATE claims SET intent = 'wire export', claimed_at = ? WHERE item_id = 'FEAT-7'`, now-1800); err != nil {
		t.Fatal(err)
	}

	// One stuck message and one ask, plus an unrelated answer.
	if _, err := f.db.Exec(`
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, 'global', 'stuck', 'env mismatch', '[]', 'normal'),
		       (?, ?, ?, 'FEAT-7', 'ask', 'is the schema settled?', '[]', 'normal')
	`, f.repoID, hourAgo, f.agentID, f.repoID, hourAgo, f.agentID); err != nil {
		t.Fatal(err)
	}

	// Active touch.
	if _, err := f.db.Exec(`
		INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
		VALUES (?, ?, 'FEAT-7', 'cmd/squad/standup.go', ?)
	`, f.repoID, f.agentID, hourAgo); err != nil {
		t.Fatal(err)
	}

	bc := &claimContext{db: f.db, agentID: f.agentID, repoID: f.repoID}
	r, err := buildStandup(ctx, bc, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Closed) != 2 {
		t.Fatalf("Closed=%v want 2 (BUG-1, FEAT-2)", r.Closed)
	}
	for _, c := range r.Closed {
		if c.ItemID == "OLD-9" {
			t.Fatalf("Closed should not include out-of-window OLD-9")
		}
	}
	if len(r.Reclaimed) != 1 || r.Reclaimed[0].ItemID != "CHORE-3" {
		t.Fatalf("Reclaimed=%v want [CHORE-3]", r.Reclaimed)
	}
	if r.OpenClaim == nil || r.OpenClaim.ItemID != "FEAT-7" || r.OpenClaim.Intent != "wire export" {
		t.Fatalf("OpenClaim=%+v want FEAT-7 wire export", r.OpenClaim)
	}
	if len(r.Stuck) != 1 || r.Stuck[0].Body != "env mismatch" {
		t.Fatalf("Stuck=%v", r.Stuck)
	}
	if len(r.UnansweredAsks) != 1 {
		t.Fatalf("UnansweredAsks=%v want 1", r.UnansweredAsks)
	}
	if len(r.ActiveTouches) != 1 || r.ActiveTouches[0].Path != "cmd/squad/standup.go" {
		t.Fatalf("ActiveTouches=%v", r.ActiveTouches)
	}
}

func TestStandup_AnsweredAskIsExcluded(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	now := time.Now().Unix()

	// I post an ask, then someone else answers within the window.
	if _, err := f.db.Exec(`
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, 'FEAT-1', 'ask', 'when does this ship?', '[]', 'normal'),
		       (?, ?, 'agent-other', 'FEAT-1', 'answer', 'thursday', '[]', 'normal')
	`, f.repoID, now-600, f.agentID, f.repoID, now-100); err != nil {
		t.Fatal(err)
	}

	bc := &claimContext{db: f.db, agentID: f.agentID, repoID: f.repoID}
	r, err := buildStandup(ctx, bc, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.UnansweredAsks) != 0 {
		t.Fatalf("answered ask should not appear in UnansweredAsks: %v", r.UnansweredAsks)
	}
}

func TestStandup_EmptyState(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	bc := &claimContext{db: f.db, agentID: f.agentID, repoID: f.repoID}
	r, err := buildStandup(ctx, bc, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Closed) != 0 || len(r.Reclaimed) != 0 || r.OpenClaim != nil ||
		len(r.Stuck) != 0 || len(r.UnansweredAsks) != 0 || len(r.ActiveTouches) != 0 {
		t.Fatalf("expected all-empty digest, got %+v", r)
	}
}
