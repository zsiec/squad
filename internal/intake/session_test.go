package intake

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

var sessionIDRE = regexp.MustCompile(`^intake-\d{8}-[0-9a-f]{12}$`)

func TestSession_OpenNewIsFresh(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	s, resumed, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "make logging searchable")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if resumed {
		t.Errorf("first open should not be resumed")
	}
	if !sessionIDRE.MatchString(s.ID) {
		t.Errorf("id %q does not match intake-YYYYMMDD-<12 hex>", s.ID)
	}
	if s.Status != StatusOpen {
		t.Errorf("status=%q want %q", s.Status, StatusOpen)
	}
	if s.IdeaSeed != "make logging searchable" {
		t.Errorf("idea_seed=%q", s.IdeaSeed)
	}
	if s.Mode != ModeNew {
		t.Errorf("mode=%q want %q", s.Mode, ModeNew)
	}
}

func TestSession_OpenTwiceFromSameAgentResumes(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	first, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "first idea")
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, resumed, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "second idea ignored")
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if !resumed {
		t.Errorf("second open should be resumed")
	}
	if second.ID != first.ID {
		t.Errorf("resumed id=%q want %q", second.ID, first.ID)
	}
	if second.IdeaSeed != "first idea" {
		t.Errorf("idea_seed should be the original; got %q", second.IdeaSeed)
	}
}

func TestSession_OpenFromDifferentAgentInSameRepoCreatesNew(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	first, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "idea-a")
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, resumed, err := Open(ctx, db, "repo-a", "agent-2", ModeNew, "idea-b")
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if resumed {
		t.Errorf("different agent must not resume")
	}
	if second.ID == first.ID {
		t.Errorf("different agent got same id %q", second.ID)
	}
}

func TestSession_OpenFromSameAgentDifferentRepoCreatesNew(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	first, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "idea-a")
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, resumed, err := Open(ctx, db, "repo-b", "agent-1", ModeNew, "idea-b")
	if err != nil {
		t.Fatalf("open 2: %v", err)
	}
	if resumed {
		t.Errorf("different repo must not resume")
	}
	if second.ID == first.ID {
		t.Errorf("different repo got same id %q", second.ID)
	}
}

func TestSession_CancelMarksClosed(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	s, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "x")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Cancel(ctx, db, s.ID, "agent-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM intake_sessions WHERE id=?`, s.ID).Scan(&status); err != nil {
		t.Fatalf("post-cancel scan: %v", err)
	}
	if status != StatusCancelled {
		t.Errorf("post-cancel status=%q want %q", status, StatusCancelled)
	}

	resumed, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "fresh")
	if err != nil {
		t.Fatalf("open after cancel: %v", err)
	}
	if resumed.ID == s.ID {
		t.Errorf("after cancel, open must allocate a new id; got the cancelled one back")
	}
}

func TestSession_CancelByOtherAgentRejected(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	s, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "x")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	err = Cancel(ctx, db, s.ID, "agent-2")
	if !errors.Is(err, ErrIntakeNotYours) {
		t.Errorf("cancel by foreign agent: err=%v, want ErrIntakeNotYours", err)
	}

	var status string
	_ = db.QueryRow(`SELECT status FROM intake_sessions WHERE id=?`, s.ID).Scan(&status)
	if status != StatusOpen {
		t.Errorf("foreign cancel mutated status: %q", status)
	}
}

func TestSession_CancelAlreadyCancelledRejected(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	s, _, err := Open(ctx, db, "repo-a", "agent-1", ModeNew, "x")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Cancel(ctx, db, s.ID, "agent-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	err = Cancel(ctx, db, s.ID, "agent-1")
	if !errors.Is(err, ErrIntakeAlreadyClosed) {
		t.Errorf("re-cancel: err=%v, want ErrIntakeAlreadyClosed", err)
	}
}

func TestSession_CancelUnknownIDReturnsNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	err := Cancel(ctx, db, "intake-19700101-deadbeefcafe", "agent-1")
	if !errors.Is(err, ErrIntakeNotFound) {
		t.Errorf("cancel unknown: err=%v, want ErrIntakeNotFound", err)
	}
}

func TestSession_RejectsUnknownMode(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, _, err := Open(ctx, db, "repo-a", "agent-1", "bogus", "x")
	if err == nil {
		t.Errorf("open with mode=bogus should error")
	}
	if !strings.Contains(err.Error(), "mode") {
		t.Errorf("error should mention mode; got %v", err)
	}
}
