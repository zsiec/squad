package hygiene

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

type emptyItems struct{}

func (emptyItems) List(context.Context) ([]ItemRef, error) { return nil, nil }

type fakeItems struct{ refs []ItemRef }

func (f fakeItems) List(context.Context) ([]ItemRef, error) { return f.refs, nil }

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func registerAgent(t *testing.T, db *sql.DB, repoID, id string, lastTickAt int64) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, '/tmp/wt', 1, ?, ?, 'active')
	`, id, repoID, id, lastTickAt, lastTickAt); err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
}

func insertClaim(t *testing.T, db *sql.DB, repoID, itemID, agentID string, lastTouch int64, long int) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, '', ?)
	`, repoID, itemID, agentID, lastTouch, lastTouch, long); err != nil {
		t.Fatalf("insert claim: %v", err)
	}
}

func joinFindings(fs []Finding) string {
	parts := make([]string, len(fs))
	for i, f := range fs {
		parts[i] = f.Message
	}
	return strings.Join(parts, "\n")
}

func TestSweep_FlagsStaleClaim(t *testing.T) {
	db := newDB(t)
	t0 := int64(1_000_000)
	registerAgent(t, db, "repo-test", "agent-a", t0)
	insertClaim(t, db, "repo-test", "BUG-X", "agent-a", t0, 0) // last_touch = t0

	// Now is t0 + 45 minutes — claim is stale (>30m).
	now := time.Unix(t0+45*60, 0)
	sw := NewWithClock(db, "repo-test", emptyItems{}, func() time.Time { return now })
	findings, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joinFindings(findings), "BUG-X") {
		t.Fatalf("no stale claim flagged: %s", joinFindings(findings))
	}
}

func TestSweep_FlagsGhostAgent(t *testing.T) {
	db := newDB(t)
	t0 := int64(1_000_000)
	registerAgent(t, db, "repo-test", "agent-zombie", t0)

	now := time.Unix(t0+25*3600, 0)
	sw := NewWithClock(db, "repo-test", emptyItems{}, func() time.Time { return now })
	findings, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joinFindings(findings), "agent-zombie") {
		t.Fatalf("ghost agent not flagged: %v", findings)
	}
}

func TestSweep_FlagsOrphanTouch(t *testing.T) {
	db := newDB(t)
	t0 := int64(1_000_000)
	registerAgent(t, db, "repo-test", "agent-a", t0)
	if _, err := db.Exec(`
		INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
		VALUES (?, ?, ?, ?, ?)
	`, "repo-test", "agent-a", "GONE-9", "ghost.go", t0); err != nil {
		t.Fatal(err)
	}

	sw := NewWithClock(db, "repo-test", emptyItems{}, func() time.Time { return time.Unix(t0, 0) })
	findings, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joinFindings(findings), "ghost.go") {
		t.Fatalf("orphan touch not flagged: %v", findings)
	}
}

func TestSweep_CleanIntegrityProducesNoFinding(t *testing.T) {
	db := newDB(t)
	sw := NewWithClock(db, "repo-test", emptyItems{}, time.Now)
	findings, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Code == "integrity_check" {
			t.Fatalf("clean DB should not produce integrity finding: %s", f.Message)
		}
	}
}

func TestSweep_FlagsDoneItemMarkedInProgress(t *testing.T) {
	db := newDB(t)
	items := fakeItems{refs: []ItemRef{
		{ID: "BUG-77", Path: ".squad/done/BUG-77.md", Status: "in_progress"},
	}}
	sw := NewWithClock(db, "repo-test", items, time.Now)
	findings, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joinFindings(findings), "BUG-77") {
		t.Fatalf("done-but-in-progress not flagged: %v", findings)
	}
}

func TestSweep_FlagsBrokenReference(t *testing.T) {
	db := newDB(t)
	items := fakeItems{refs: []ItemRef{
		{ID: "FEAT-1", Path: ".squad/items/FEAT-1.md", Status: "ready",
			References: []string{"/nonexistent/file.go:42"}},
	}}
	sw := NewWithClock(db, "repo-test", items, time.Now)
	findings, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(joinFindings(findings), "/nonexistent/file.go") {
		t.Fatalf("broken ref not flagged: %v", findings)
	}
}

func TestReclaimStale_ShortClaim(t *testing.T) {
	db := newDB(t)
	t0 := int64(1_000_000)
	registerAgent(t, db, "repo-test", "agent-a", t0)
	insertClaim(t, db, "repo-test", "BUG-200", "agent-a", t0, 0)

	now := time.Unix(t0+31*60, 0)
	sw := NewWithClock(db, "repo-test", emptyItems{}, func() time.Time { return now })
	got, err := sw.ReclaimStale(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "BUG-200" {
		t.Fatalf("got=%v want [BUG-200]", got)
	}
	var c int
	_ = db.QueryRow(`SELECT COUNT(*) FROM claims WHERE item_id='BUG-200'`).Scan(&c)
	if c != 0 {
		t.Fatalf("claim still present")
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM claim_history WHERE item_id='BUG-200' AND outcome='reclaimed'`).Scan(&c)
	if c != 1 {
		t.Fatalf("history row missing")
	}
}

func TestReclaimStale_LongClaimWaitsTwoHours(t *testing.T) {
	db := newDB(t)
	t0 := int64(1_000_000)
	registerAgent(t, db, "repo-test", "agent-a", t0)
	insertClaim(t, db, "repo-test", "BUG-201", "agent-a", t0, 1) // long=1

	clock := time.Unix(t0+31*60, 0)
	sw := NewWithClock(db, "repo-test", emptyItems{}, func() time.Time { return clock })
	got, _ := sw.ReclaimStale(context.Background())
	if len(got) != 0 {
		t.Fatalf("long claim reclaimed early: %v", got)
	}

	clock = time.Unix(t0+121*60, 0)
	got, _ = sw.ReclaimStale(context.Background())
	if len(got) != 1 {
		t.Fatalf("long claim not reclaimed after 2h: %v", got)
	}
}

func TestStripLineSuffix(t *testing.T) {
	cases := map[string]string{
		"path/foo.go:42":  "path/foo.go",
		"path/foo.go":     "path/foo.go",
		"path/foo.go:abc": "path/foo.go:abc",
		"path/foo.go:":    "path/foo.go:",
	}
	for in, want := range cases {
		if got := stripLineSuffix(in); got != want {
			t.Errorf("stripLineSuffix(%q)=%q want %q", in, got, want)
		}
	}
}
