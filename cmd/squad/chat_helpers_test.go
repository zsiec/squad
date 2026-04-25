package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/store"
)

// chatFixture spins up an isolated SQLite store with one registered agent
// and returns a *chat.Chat wired to it. Inline test helper to avoid adding
// surface to the store package.
type chatFixture struct {
	chat    *chat.Chat
	db      *sql.DB
	agentID string
	repoID  string
}

func newChatFixture(t *testing.T) *chatFixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const repoID = "repo-test"
	const agentID = "agent-test"
	clock := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	c := chat.NewWithClock(db, repoID, func() time.Time { return clock })

	if _, err := db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, 'Tester', '/tmp/wt', 1, ?, ?, 'active')
	`, agentID, repoID, clock.Unix(), clock.Unix()); err != nil {
		t.Fatalf("register agent: %v", err)
	}

	return &chatFixture{chat: c, db: db, agentID: agentID, repoID: repoID}
}

func (f *chatFixture) firstMessage(t *testing.T) (thread, kind, body, mentions, priority string) {
	t.Helper()
	if err := f.db.QueryRowContext(context.Background(),
		`SELECT thread, kind, body, mentions, priority FROM messages ORDER BY id LIMIT 1`,
	).Scan(&thread, &kind, &body, &mentions, &priority); err != nil {
		t.Fatalf("scan first message: %v", err)
	}
	return
}

func (f *chatFixture) messageCount(t *testing.T) int {
	t.Helper()
	var n int
	_ = f.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM messages`).Scan(&n)
	return n
}

func (f *chatFixture) insertClaim(t *testing.T, itemID string) {
	t.Helper()
	if _, err := f.db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, '', 0)
	`, f.repoID, itemID, f.agentID, time.Now().Unix(), time.Now().Unix()); err != nil {
		t.Fatalf("insert claim: %v", err)
	}
}
