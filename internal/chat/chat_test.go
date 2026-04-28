package chat

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

// newTestChat opens a temp SQLite DB, registers a baseline agent, and
// returns a Chat wired to it with a fixed clock. Inline test helper —
// no store-package surface area is added for this.
func newTestChat(t *testing.T) (*Chat, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	clock := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	c := NewWithClock(db, "repo-test", func() time.Time { return clock })

	if err := registerTestAgent(context.Background(), db, "repo-test", "agent-a", "Agent A", clock.Unix()); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	return c, db
}

func registerTestAgent(ctx context.Context, db *sql.DB, repoID, id, name string, ts int64) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, ?, 1, ?, ?, 'active')
	`, id, repoID, name, "/tmp/wt", ts, ts)
	return err
}

func insertTestClaim(ctx context.Context, db *sql.DB, repoID, itemID, agentID, intent string, claimedAt int64) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, repoID, itemID, agentID, claimedAt, claimedAt, intent)
	return err
}

func TestChat_NewWiresStoreAndBus(t *testing.T) {
	c, _ := newTestChat(t)
	if c.Bus() == nil {
		t.Fatal("bus must be non-nil")
	}
	if c.DB() == nil {
		t.Fatal("db must be non-nil")
	}
	if c.RepoID() != "repo-test" {
		t.Fatalf("repo=%q", c.RepoID())
	}
}

func TestKinds_AllKnown(t *testing.T) {
	known := map[string]bool{
		"say": true, "ask": true,
		"thinking": true, "stuck": true, "milestone": true, "fyi": true,
		"handoff": true, "review_req": true,
		"progress": true, "done": true, "system": true,
	}
	for _, k := range AllKinds() {
		if !known[k] {
			t.Errorf("AllKinds returned unknown kind %q", k)
		}
		delete(known, k)
	}
	if len(known) != 0 {
		t.Errorf("AllKinds missed: %v", known)
	}
}
