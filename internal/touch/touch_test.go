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

func TestAdd_RejectsOversizedPath(t *testing.T) {
	tr, db := newTestTracker(t)
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())

	huge := make([]byte, MaxPathLen+1)
	for i := range huge {
		huge[i] = 'a'
	}
	if _, err := tr.Add(context.Background(), "agent-a", "", string(huge)); err != ErrPathTooLong {
		t.Fatalf("err=%v want ErrPathTooLong", err)
	}
}

func TestAdd_ReportsConflictWithOtherAgent(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())
	registerAgent(t, db, "repo-test", "agent-b", "B", tr.nowUnix())

	if _, err := tr.Add(ctx, "agent-a", "", "shared/file.go"); err != nil {
		t.Fatal(err)
	}
	conflicts, err := tr.Add(ctx, "agent-b", "", "shared/file.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || conflicts[0] != "agent-a" {
		t.Fatalf("conflicts=%v want [agent-a]", conflicts)
	}
}

func TestRelease_ClearsSinglePath(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())

	_, _ = tr.Add(ctx, "agent-a", "", "a.go")
	_, _ = tr.Add(ctx, "agent-a", "", "b.go")
	if err := tr.Release(ctx, "agent-a", "a.go"); err != nil {
		t.Fatal(err)
	}
	var open int
	_ = db.QueryRow(`SELECT COUNT(*) FROM touches WHERE agent_id='agent-a' AND released_at IS NULL`).Scan(&open)
	if open != 1 {
		t.Fatalf("open=%d want 1", open)
	}
}

func TestReleaseAll_ClearsEveryPath(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())

	_, _ = tr.Add(ctx, "agent-a", "", "a.go")
	_, _ = tr.Add(ctx, "agent-a", "", "b.go")
	_, _ = tr.Add(ctx, "agent-a", "", "c.go")
	n, err := tr.ReleaseAll(ctx, "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("released=%d want 3", n)
	}
}

func TestConflicts_ReadOnly(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())
	registerAgent(t, db, "repo-test", "agent-b", "B", tr.nowUnix())

	_, _ = tr.Add(ctx, "agent-a", "", "shared.go")

	got, err := tr.Conflicts(ctx, "agent-b", "shared.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "agent-a" {
		t.Fatalf("conflicts=%v want [agent-a]", got)
	}
	// Conflicts must not insert.
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM touches WHERE agent_id='agent-b' AND released_at IS NULL`).Scan(&n)
	if n != 0 {
		t.Fatalf("Conflicts() inserted rows: %d", n)
	}
}
