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

// registerRepo seeds the repos row that touch.normalizePath consults
// when stripping repo-root prefixes off absolute paths.
func registerRepo(t *testing.T, db *sql.DB, repoID, rootPath string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO repos (id, root_path, remote_url, name, created_at) VALUES (?, ?, '', ?, 0)`,
		repoID, rootPath, repoID,
	); err != nil {
		t.Fatalf("register repo: %v", err)
	}
}

// TestAdd_AbsoluteAndRelativePathsCollide is the regression for the
// path-normalization bug — the pre-edit-touch hook records absolute
// paths (Claude Code emits tool_input.file_path absolute) while squad
// touch and squad claim --touches=... pass repo-relative paths. The
// touches table treated them as distinct rows; this test pins that
// they collide on the conflict query.
func TestAdd_AbsoluteAndRelativePathsCollide(t *testing.T) {
	tr, db := newTestTracker(t)
	root := t.TempDir()
	registerRepo(t, db, "repo-test", root)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-hook", "Hook", tr.nowUnix())
	registerAgent(t, db, "repo-test", "agent-cli", "CLI", tr.nowUnix())

	abs := filepath.Join(root, "internal/foo/bar.go")
	if _, err := tr.Add(ctx, "agent-hook", "", abs); err != nil {
		t.Fatalf("Add abs: %v", err)
	}

	conflicts, err := tr.Add(ctx, "agent-cli", "", "internal/foo/bar.go")
	if err != nil {
		t.Fatalf("Add rel: %v", err)
	}
	if len(conflicts) != 1 || conflicts[0] != "agent-hook" {
		t.Fatalf("conflicts=%v want [agent-hook] — abs+rel writes must collide", conflicts)
	}

	others, err := tr.Conflicts(ctx, "agent-cli", "internal/foo/bar.go")
	if err != nil {
		t.Fatalf("Conflicts rel: %v", err)
	}
	if len(others) != 1 || others[0] != "agent-hook" {
		t.Errorf("Conflicts rel→abs match failed: %v", others)
	}

	// `./internal/foo/bar.go` is the same logical file; filepath.Clean
	// drops the `./` prefix so it must collide too. Cheap insurance
	// against Clean's semantics drifting.
	dot, err := tr.Conflicts(ctx, "agent-cli", "./internal/foo/bar.go")
	if err != nil {
		t.Fatalf("Conflicts ./rel: %v", err)
	}
	if len(dot) != 1 || dot[0] != "agent-hook" {
		t.Errorf("`./` prefix should collapse to the canonical relative; got %v", dot)
	}

	if err := tr.Release(ctx, "agent-hook", abs); err != nil {
		t.Fatalf("Release abs: %v", err)
	}
	stillThere, _ := tr.Conflicts(ctx, "agent-cli", "internal/foo/bar.go")
	if len(stillThere) != 0 {
		t.Errorf("Release abs should clear the rel-form conflict; got %v", stillThere)
	}
}

// TestNormalizePath_OutsideRepoStaysAbsolute covers the fallback
// branch — a vendored dep or system path outside the repo root cannot
// be expressed as a clean relative form, so it stays absolute.
func TestNormalizePath_OutsideRepoStaysAbsolute(t *testing.T) {
	tr, db := newTestTracker(t)
	root := t.TempDir()
	registerRepo(t, db, "repo-test", root)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())

	outside := "/usr/local/share/external.go"
	if _, err := tr.Add(ctx, "agent-a", "", outside); err != nil {
		t.Fatal(err)
	}
	var stored string
	_ = db.QueryRow(`SELECT path FROM touches WHERE agent_id='agent-a' LIMIT 1`).Scan(&stored)
	if stored != outside {
		t.Errorf("path outside repo should stay absolute; stored=%q want %q", stored, outside)
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

func TestListOthersSince_FiltersByFreshness(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())
	registerAgent(t, db, "repo-test", "agent-b", "B", tr.nowUnix())

	now := tr.now()
	_, err := db.Exec(`INSERT INTO touches (repo_id, agent_id, item_id, path, started_at) VALUES (?, ?, ?, ?, ?)`,
		"repo-test", "agent-a", "FEAT-1", "fresh.go", now.Add(-1*time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO touches (repo_id, agent_id, item_id, path, started_at) VALUES (?, ?, ?, ?, ?)`,
		"repo-test", "agent-a", "FEAT-2", "stale.go", now.Add(-72*time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}

	got, err := tr.ListOthersSince(ctx, "agent-b", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d touches, want 1 (only fresh.go): %+v", len(got), got)
	}
	if got[0].Path != "fresh.go" {
		t.Errorf("path=%q want fresh.go", got[0].Path)
	}
	if got[0].StartedAt != now.Add(-1*time.Hour).Unix() {
		t.Errorf("started_at=%d want %d", got[0].StartedAt, now.Add(-1*time.Hour).Unix())
	}
}

func TestListOthersSince_ExcludesCallerOwnTouches(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())
	registerAgent(t, db, "repo-test", "agent-b", "B", tr.nowUnix())

	if _, err := tr.Add(ctx, "agent-b", "FEAT-1", "own.go"); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Add(ctx, "agent-a", "FEAT-2", "peer.go"); err != nil {
		t.Fatal(err)
	}
	got, err := tr.ListOthersSince(ctx, "agent-b", tr.now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "peer.go" {
		t.Fatalf("ListOthersSince must exclude caller's own touches; got %+v", got)
	}
}

func TestListOthersSince_ExcludesReleasedTouches(t *testing.T) {
	tr, db := newTestTracker(t)
	ctx := context.Background()
	registerAgent(t, db, "repo-test", "agent-a", "A", tr.nowUnix())
	registerAgent(t, db, "repo-test", "agent-b", "B", tr.nowUnix())

	if _, err := tr.Add(ctx, "agent-a", "FEAT-1", "released.go"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Release(ctx, "agent-a", "released.go"); err != nil {
		t.Fatal(err)
	}
	got, err := tr.ListOthersSince(ctx, "agent-b", tr.now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("released touches must not surface; got %+v", got)
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
