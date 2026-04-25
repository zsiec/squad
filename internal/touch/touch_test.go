package touch

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func newTestTracker(t *testing.T) (*Tracker, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	return NewWithClock(db, "repo-test", func() time.Time { return clock }), db
}

func registerAgent(t *testing.T, db *sql.DB, repoID, id, name string, ts int64) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, '/tmp/wt', 1, ?, ?, 'active')
	`, id, repoID, name, ts, ts); err != nil {
		t.Fatalf("register agent %s: %v", id, err)
	}
}

func TestAdd_InsertsRow_NoConflicts(t *testing.T) {
	tr, db := newTestTracker(t)
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())

	conflicts, err := tr.Add(context.Background(), "agent-a", "", "internal/foo/bar.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("conflicts=%v want empty", conflicts)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM touches WHERE agent_id=? AND released_at IS NULL`, "agent-a").Scan(&n)
	if n != 1 {
		t.Fatalf("rows=%d want 1", n)
	}
}
