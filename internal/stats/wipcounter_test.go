package stats

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRecordWIPViolationIncrements(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := RecordWIPViolation(ctx, db, "repo-1", "agent-a", 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	n, _ := CountWIPViolations(ctx, db, "repo-1", 0, 0)
	if n != 3 {
		t.Errorf("count: %d", n)
	}
}

func TestRecordWIPViolationFiltersByAgentAndRepo(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	_ = RecordWIPViolation(ctx, db, "repo-1", "agent-a", 1, 1)
	_ = RecordWIPViolation(ctx, db, "repo-1", "agent-b", 1, 1)
	_ = RecordWIPViolation(ctx, db, "repo-2", "agent-a", 1, 1)
	if n, _ := CountWIPViolationsByAgent(ctx, db, "repo-1", "agent-a", 0, 0); n != 1 {
		t.Errorf("agent-a@repo-1: %d", n)
	}
	if n, _ := CountWIPViolations(ctx, db, "repo-1", 0, 0); n != 2 {
		t.Errorf("repo-1: %d", n)
	}
}

// Headline test required by the phase scope: a claim attempt at cap+1
// increments the counter.
func TestRecordWIPViolationAtCapPlusOne(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	cap := int64(5)
	if err := RecordWIPViolation(ctx, db, "repo-1", "agent-a", cap, cap); err != nil {
		t.Fatal(err)
	}
	if n, _ := CountWIPViolations(ctx, db, "repo-1", 0, 0); n != 1 {
		t.Fatalf("expected 1 violation, got %d", n)
	}
	var held, capStored int64
	_ = db.QueryRow(`SELECT held_at_attempt, cap_at_attempt FROM wip_violations
		WHERE repo_id='repo-1'`).Scan(&held, &capStored)
	if held != cap || capStored != cap {
		t.Errorf("held=%d cap=%d", held, capStored)
	}
}
