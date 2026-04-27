package intake

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

const testRepoID = "repo-test"

func setupSquadDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	squadDir := filepath.Join(root, ".squad")
	if err := os.MkdirAll(filepath.Join(squadDir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	return squadDir
}

func openSessionForCommit(t *testing.T, db *sql.DB, squadDir, agentID string) Session {
	t.Helper()
	s, _, _, err := Open(context.Background(), db, OpenParams{
		RepoID: testRepoID, AgentID: agentID, Mode: ModeNew, IdeaSeed: "x",
		SquadDir: squadDir,
	})
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	return s
}

func validItemOnlyBundle() Bundle {
	return Bundle{
		Items: []ItemDraft{
			{
				Title:      "rotate keys without downtime",
				Intent:     "support online rotation",
				Acceptance: []string{"keys rotate", "no requests fail"},
				Area:       "auth",
			},
			{
				Title:      "verify rotation in tests",
				Intent:     "regression coverage",
				Acceptance: []string{"unit + integration"},
				Area:       "auth",
			},
		},
	}
}

func TestCommit_ItemOnlyHappyPath(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	res, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", validItemOnlyBundle(), false)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if res.Shape != ShapeItemOnly {
		t.Errorf("shape=%q want %q", res.Shape, ShapeItemOnly)
	}
	if len(res.ItemIDs) != 2 || len(res.Paths) != 2 {
		t.Fatalf("ids=%v paths=%v", res.ItemIDs, res.Paths)
	}
	for _, p := range res.Paths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file missing on disk: %s (%v)", p, err)
		}
	}

	for _, id := range res.ItemIDs {
		var status, sessionLink string
		err := db.QueryRow(
			`SELECT status, COALESCE(intake_session_id,'') FROM items WHERE repo_id=? AND item_id=?`,
			testRepoID, id,
		).Scan(&status, &sessionLink)
		if err != nil {
			t.Fatalf("scan %s: %v", id, err)
		}
		if status != "captured" {
			t.Errorf("%s status=%q want captured", id, status)
		}
		if sessionLink != sess.ID {
			t.Errorf("%s intake_session_id=%q want %q", id, sessionLink, sess.ID)
		}
	}

	var sessStatus, shape, bundleJSON string
	var committedAt sql.NullInt64
	err = db.QueryRow(`SELECT status, COALESCE(shape,''), COALESCE(bundle_json,''), committed_at
		FROM intake_sessions WHERE id=?`, sess.ID).Scan(&sessStatus, &shape, &bundleJSON, &committedAt)
	if err != nil {
		t.Fatalf("session row: %v", err)
	}
	if sessStatus != StatusCommitted {
		t.Errorf("session.status=%q want committed", sessStatus)
	}
	if shape != ShapeItemOnly {
		t.Errorf("session.shape=%q", shape)
	}
	if !committedAt.Valid {
		t.Errorf("committed_at not set")
	}
	var roundTrip Bundle
	if err := json.Unmarshal([]byte(bundleJSON), &roundTrip); err != nil {
		t.Fatalf("bundle_json not valid JSON: %v", err)
	}
	if len(roundTrip.Items) != 2 {
		t.Errorf("bundle_json round-trip items=%d want 2", len(roundTrip.Items))
	}
}

func TestCommit_ReadyFlagPromotesStatus(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	bundle := Bundle{Items: []ItemDraft{{
		Title: "single ready item", Intent: "x",
		Acceptance: []string{"ok"}, Area: "core",
	}}}
	res, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, true)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	var status string
	if err := db.QueryRow(
		`SELECT status FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, res.ItemIDs[0],
	).Scan(&status); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if status != "open" {
		t.Errorf("ready=true item status=%q want open", status)
	}
}

func TestCommit_FailureRollsBackFilesAndRows(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	calls := 0
	var firstPath string
	failingWriter := func(squadDir, prefix, title string, opts items.Options) (string, error) {
		calls++
		if calls == 1 {
			path, err := items.NewWithOptions(squadDir, prefix, title, opts)
			if err != nil {
				return "", err
			}
			firstPath = path
			return path, nil
		}
		return "", errors.New("synthetic write failure on item 2")
	}

	_, err := commitImpl(ctx, db, squadDir, sess.ID, "agent-1", validItemOnlyBundle(), false, failingWriter)
	if err == nil || !strings.Contains(err.Error(), "synthetic write failure") {
		t.Fatalf("want synthetic write failure surfaced; got %v", err)
	}

	if firstPath == "" {
		t.Fatal("first item path was never recorded")
	}
	if _, statErr := os.Stat(firstPath); !os.IsNotExist(statErr) {
		t.Errorf("first item file should be deleted on rollback; stat=%v", statErr)
	}

	var rowCount int
	if err := db.QueryRow(`SELECT count(*) FROM items WHERE repo_id=?`, testRepoID).Scan(&rowCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if rowCount != 0 {
		t.Errorf("items rows persisted on failure: %d", rowCount)
	}

	var sessStatus string
	if err := db.QueryRow(`SELECT status FROM intake_sessions WHERE id=?`, sess.ID).Scan(&sessStatus); err != nil {
		t.Fatalf("session: %v", err)
	}
	if sessStatus != StatusOpen {
		t.Errorf("session status mutated on failure: %q", sessStatus)
	}
}

func TestCommit_AlreadyCommittedRejected(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	if _, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", validItemOnlyBundle(), false); err != nil {
		t.Fatalf("first commit: %v", err)
	}
	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", validItemOnlyBundle(), false)
	if !errors.Is(err, ErrIntakeAlreadyClosed) {
		t.Errorf("re-commit: err=%v, want ErrIntakeAlreadyClosed", err)
	}
}

func TestCommit_ForeignAgentRejected(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-2", validItemOnlyBundle(), false)
	if !errors.Is(err, ErrIntakeNotYours) {
		t.Errorf("foreign commit: err=%v, want ErrIntakeNotYours", err)
	}
}

func TestCommit_UnknownSessionReturnsNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)

	_, err := Commit(ctx, db, squadDir, "intake-19700101-deadbeefcafe", "agent-1",
		validItemOnlyBundle(), false)
	if !errors.Is(err, ErrIntakeNotFound) {
		t.Errorf("unknown commit: err=%v, want ErrIntakeNotFound", err)
	}
}

// seedCapturedItem writes a captured-shaped item file at
// <squadDir>/items/<id>-<slug>.md and inserts the matching items row.
// Returns the absolute file path.
func seedCapturedItem(t *testing.T, db *sql.DB, squadDir, id, title, body string) string {
	t.Helper()
	path := filepath.Join(squadDir, "items", id+"-rotate-keys.md")
	frontmatter := "---\n" +
		"id: " + id + "\n" +
		"title: " + title + "\n" +
		"type: feature\n" +
		"priority: P2\n" +
		"area: auth\n" +
		"status: captured\n" +
		"estimate: 1h\n" +
		"risk: low\n" +
		"created: 2026-04-26\n" +
		"updated: 2026-04-26\n" +
		"captured_by: agent-1\n" +
		"captured_at: 1714150000\n" +
		"---\n\n"
	if err := os.WriteFile(path, []byte(frontmatter+body), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := items.Parse(path)
	if err != nil {
		t.Fatalf("parse seeded: %v", err)
	}
	if err := items.Persist(context.Background(), db, testRepoID, parsed, false); err != nil {
		t.Fatalf("persist seeded: %v", err)
	}
	return path
}

func openRefineSession(t *testing.T, db *sql.DB, squadDir, agentID, refineItemID string) Session {
	t.Helper()
	s, _, _, err := Open(context.Background(), db, OpenParams{
		RepoID: testRepoID, AgentID: agentID, Mode: ModeRefine,
		IdeaSeed: "follow-up", RefineItemID: refineItemID, SquadDir: squadDir,
	})
	if err != nil {
		t.Fatalf("open refine: %v", err)
	}
	return s
}

func TestCommit_RefineArchivesOriginalAndRecordsHistory(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)

	origPath := seedCapturedItem(t, db, squadDir, "FEAT-100", "rotate keys",
		"## Problem\nold problem\n")
	sess := openRefineSession(t, db, squadDir, "agent-1", "FEAT-100")

	bundle := Bundle{Items: []ItemDraft{{
		Title: "rotate keys cleanly", Intent: "online rotation",
		Acceptance: []string{"keys rotate", "no downtime"}, Area: "auth",
	}}}
	res, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, false)
	if err != nil {
		t.Fatalf("refine commit: %v", err)
	}
	if len(res.Paths) != 1 {
		t.Fatalf("paths=%v want 1", res.Paths)
	}

	if _, statErr := os.Stat(origPath); !os.IsNotExist(statErr) {
		t.Errorf("original file still at %s; expected move", origPath)
	}
	archivePath := filepath.Join(squadDir, "items", ".archive", "FEAT-100-rotate-keys.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archived file missing at %s: %v", archivePath, err)
	}

	var status string
	var archived int
	var path string
	if err := db.QueryRow(
		`SELECT status, archived, path FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-100",
	).Scan(&status, &archived, &path); err != nil {
		t.Fatalf("scan original row: %v", err)
	}
	if status != "done" {
		t.Errorf("original status=%q want done", status)
	}
	if archived != 1 {
		t.Errorf("original archived=%d want 1", archived)
	}
	if path != archivePath {
		t.Errorf("original path=%q want %q", path, archivePath)
	}

	var hist int
	if err := db.QueryRow(
		`SELECT count(*) FROM claim_history WHERE repo_id=? AND item_id=? AND outcome=?`,
		testRepoID, "FEAT-100", refineSupersededOutcome,
	).Scan(&hist); err != nil {
		t.Fatalf("count claim_history: %v", err)
	}
	if hist != 1 {
		t.Errorf("claim_history rows for FEAT-100/superseded=%d want 1", hist)
	}
}

func TestCommit_RefineMultiItemBundleRejected(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	origPath := seedCapturedItem(t, db, squadDir, "FEAT-101", "rotate keys", "## Problem\n")
	sess := openRefineSession(t, db, squadDir, "agent-1", "FEAT-101")

	bundle := Bundle{Items: []ItemDraft{
		{Title: "a", Intent: "x", Acceptance: []string{"ok"}, Area: "core"},
		{Title: "b", Intent: "x", Acceptance: []string{"ok"}, Area: "core"},
	}}
	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, false)
	if err == nil {
		t.Fatal("want error for multi-item refine bundle")
	}
	var shapeErr *IntakeShapeInvalid
	if !errors.As(err, &shapeErr) {
		t.Errorf("multi-item refine: err=%T %v, want *IntakeShapeInvalid", err, err)
	}

	if _, statErr := os.Stat(origPath); statErr != nil {
		t.Errorf("original moved despite rejection: %v", statErr)
	}
	archivePath := filepath.Join(squadDir, "items", ".archive", "FEAT-101-rotate-keys.md")
	if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
		t.Errorf("archive populated despite rejection: %v", statErr)
	}

	var hist int
	_ = db.QueryRow(`SELECT count(*) FROM claim_history WHERE item_id=?`, "FEAT-101").Scan(&hist)
	if hist != 0 {
		t.Errorf("claim_history written despite rejection: %d", hist)
	}
}

func TestCommit_RefineRejectsIfOriginalNoLongerCaptured(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	origPath := seedCapturedItem(t, db, squadDir, "FEAT-102", "rotate keys", "## Problem\n")
	sess := openRefineSession(t, db, squadDir, "agent-1", "FEAT-102")

	mutated := strings.Replace(string(mustRead(t, origPath)),
		"status: captured", "status: open", 1)
	if err := os.WriteFile(origPath, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle := Bundle{Items: []ItemDraft{{
		Title: "x", Intent: "y", Acceptance: []string{"ok"}, Area: "core",
	}}}
	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, false)
	if !errors.Is(err, ErrIntakeItemNotRefinable) {
		t.Errorf("err=%v, want ErrIntakeItemNotRefinable", err)
	}

	if _, statErr := os.Stat(origPath); statErr != nil {
		t.Errorf("original moved on rejection: %v", statErr)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCommit_RejectsSpecEpicShapeForNow(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	bundle := Bundle{
		Spec: &SpecDraft{Title: "x"},
		Epics: []EpicDraft{{Title: "e"}},
		Items: []ItemDraft{{Title: "y", Intent: "z", Acceptance: []string{"a"}, Area: "core", Epic: "e"}},
	}
	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, false)
	if err == nil {
		t.Fatal("want error for spec_epic_items shape; FEAT-022 hasn't landed yet")
	}
}
