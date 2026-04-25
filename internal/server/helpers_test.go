package server

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

const testRepoID = "repo-test"

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func registerAgent(t *testing.T, db *sql.DB, id, name string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, '/tmp/wt', 1, 0, 0, 'active')
	`, id, testRepoID, name); err != nil {
		t.Fatalf("register agent: %v", err)
	}
}

func insertClaim(t *testing.T, db *sql.DB, agentID, itemID, intent string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, 0, 0, ?, 0)
	`, testRepoID, itemID, agentID, intent); err != nil {
		t.Fatalf("insert claim: %v", err)
	}
}

func insertRepo(t *testing.T, db *sql.DB, id, root, remote string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO repos (id, root_path, remote_url, name, created_at) VALUES (?, ?, ?, '', 0)
	`, id, root, remote); err != nil {
		t.Fatalf("insert repo: %v", err)
	}
}
