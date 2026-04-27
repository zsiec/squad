package intake

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
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

	s, _, resumed, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "make logging searchable"})
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

	first, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "first idea"})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, _, resumed, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "second idea ignored"})
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

	first, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "idea-a"})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, _, resumed, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-2", Mode: ModeNew, IdeaSeed: "idea-b"})
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

	first, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "idea-a"})
	if err != nil {
		t.Fatalf("open 1: %v", err)
	}
	second, _, resumed, err := Open(ctx, db, OpenParams{RepoID: "repo-b", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "idea-b"})
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

	s, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "x"})
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

	resumed, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "fresh"})
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

	s, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "x"})
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

	s, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "x"})
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
	_, _, _, err := Open(ctx, db, OpenParams{RepoID: "repo-a", AgentID: "agent-1", Mode: "bogus", IdeaSeed: "x"})
	if err == nil {
		t.Errorf("open with mode=bogus should error")
	}
	if !strings.Contains(err.Error(), "mode") {
		t.Errorf("error should mention mode; got %v", err)
	}
}

// writeItem writes a captured-shaped (or other-status) item into
// <squadDir>/<sub>/<id>.md, mirroring how `squad new` lays them out.
// Returns the squadDir.
func writeItem(t *testing.T, sub, id, status, title, body string) string {
	t.Helper()
	squadDir := t.TempDir()
	dir := filepath.Join(squadDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	frontmatter := "---\n" +
		"id: " + id + "\n" +
		"title: " + title + "\n" +
		"type: feature\n" +
		"priority: P2\n" +
		"area: auth\n" +
		"status: " + status + "\n" +
		"estimate: 1h\n" +
		"risk: low\n" +
		"created: 2026-04-26\n" +
		"updated: 2026-04-26\n" +
		"captured_by: agent-9f3a\n" +
		"captured_at: 1714150000\n" +
		"parent_spec: auth-rotation\n" +
		"parent_epic: rotation-rollout\n" +
		"---\n\n"
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(frontmatter+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return squadDir
}

func TestSession_OpenRefineHydratesSnapshot(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	body := "## Problem\nrotate keys without downtime\n"
	squadDir := writeItem(t, "items", "FEAT-100", "captured", "rotate keys", body)

	s, snap, resumed, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		IdeaSeed: "follow-up", RefineItemID: "FEAT-100", SquadDir: squadDir,
	})
	if err != nil {
		t.Fatalf("open refine: %v", err)
	}
	if resumed {
		t.Errorf("first refine open should not be resumed")
	}
	if s.Mode != ModeRefine {
		t.Errorf("session.Mode=%q want %q", s.Mode, ModeRefine)
	}
	if s.RefineItemID != "FEAT-100" {
		t.Errorf("session.RefineItemID=%q want FEAT-100", s.RefineItemID)
	}
	if snap.ID != "FEAT-100" || snap.Title != "rotate keys" {
		t.Errorf("snapshot id=%q title=%q want FEAT-100/rotate keys", snap.ID, snap.Title)
	}
	if snap.Area != "auth" {
		t.Errorf("snapshot.Area=%q want auth", snap.Area)
	}
	if snap.Status != "captured" {
		t.Errorf("snapshot.Status=%q want captured", snap.Status)
	}
	if snap.ParentSpec != "auth-rotation" || snap.ParentEpic != "rotation-rollout" {
		t.Errorf("snapshot.ParentSpec=%q ParentEpic=%q", snap.ParentSpec, snap.ParentEpic)
	}
	if !strings.Contains(snap.Body, "rotate keys without downtime") {
		t.Errorf("snapshot.Body did not pick up the markdown body: %q", snap.Body)
	}

	// Verify the row persisted refine_item_id.
	var got sql.NullString
	if err := db.QueryRow(`SELECT refine_item_id FROM intake_sessions WHERE id=?`, s.ID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.String != "FEAT-100" {
		t.Errorf("persisted refine_item_id=%v, want FEAT-100", got)
	}
}

func TestSession_OpenRefineRejectsMissingItem(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := t.TempDir()

	_, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		RefineItemID: "FEAT-404", SquadDir: squadDir,
	})
	if !errors.Is(err, items.ErrItemNotFound) {
		t.Fatalf("expected items.ErrItemNotFound; got %v", err)
	}
	// Ensure no row was inserted on the rejection.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM intake_sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("inserted %d session rows on rejected refine; want 0", n)
	}
}

func TestSession_OpenRefineRejectsClaimedItem(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := writeItem(t, "items", "FEAT-101", "claimed", "claimed item", "## Problem\n")

	_, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		RefineItemID: "FEAT-101", SquadDir: squadDir,
	})
	if !errors.Is(err, ErrIntakeItemNotRefinable) {
		t.Fatalf("expected ErrIntakeItemNotRefinable for claimed item; got %v", err)
	}
}

func TestSession_OpenRefineRejectsDoneItem(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := writeItem(t, "done", "FEAT-102", "done", "done item", "## Problem\n")

	_, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		RefineItemID: "FEAT-102", SquadDir: squadDir,
	})
	if !errors.Is(err, ErrIntakeItemNotRefinable) {
		t.Fatalf("expected ErrIntakeItemNotRefinable for done item; got %v", err)
	}
}

func TestSession_OpenRefineAcceptsNeedsRefinement(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := writeItem(t, "items", "FEAT-200", "needs-refinement", "tighten ac", "## Problem\nfoo\n")

	_, snap, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		RefineItemID: "FEAT-200", SquadDir: squadDir,
	})
	if err != nil {
		t.Fatalf("open refine on needs-refinement item: %v", err)
	}
	if snap.Status != "needs-refinement" {
		t.Errorf("snapshot.Status=%q want needs-refinement", snap.Status)
	}
}
