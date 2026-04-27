package intake

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validSpecEpicItemsBundle() Bundle {
	return Bundle{
		Spec: &SpecDraft{
			Title:       "auth rotation overhaul",
			Motivation:  "rotate signing keys without downtime",
			Acceptance:  []string{"keys rotate online", "no failed requests during rotation"},
			NonGoals:    []string{"changing signing algorithm"},
			Integration: []string{"auth middleware", "session store"},
		},
		Epics: []EpicDraft{
			{
				Title:        "core rotation primitives",
				Parallelism:  "serial: store changes, then issuer changes",
				Dependencies: []string{},
			},
			{
				Title:        "rotation observability",
				Parallelism:  "parallel with core",
				Dependencies: []string{},
			},
		},
		Items: []ItemDraft{
			{
				Title:      "session store dual-read",
				Intent:     "tolerate both old and new key in store",
				Acceptance: []string{"store accepts dual sigs", "tests cover overlap"},
				Area:       "auth",
				Epic:       "core rotation primitives",
			},
			{
				Title:      "issuer key roll endpoint",
				Intent:     "operator-triggered roll",
				Acceptance: []string{"POST /admin/keys/roll returns 202", "audit log entry"},
				Area:       "auth",
				Epic:       "core rotation primitives",
			},
			{
				Title:      "rotation metrics",
				Intent:     "expose key-age and roll-count",
				Acceptance: []string{"squad_keys_age_seconds gauge", "squad_keys_rolls_total counter"},
				Area:       "observability",
				Epic:       "rotation observability",
			},
		},
	}
}

func TestCommit_SpecEpicItems_HappyPath_AllLinkagesPresent(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	res, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", validSpecEpicItemsBundle(), false)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	if res.Shape != ShapeSpecEpicItems {
		t.Errorf("shape=%q want %q", res.Shape, ShapeSpecEpicItems)
	}
	if len(res.ItemIDs) != 3 {
		t.Errorf("ids=%v want 3", res.ItemIDs)
	}

	specSlug := slugFromTitle("auth rotation overhaul")
	specPath := filepath.Join(squadDir, "specs", specSlug+".md")
	if _, err := os.Stat(specPath); err != nil {
		t.Errorf("spec file missing on disk: %v", err)
	}
	for _, epicTitle := range []string{"core rotation primitives", "rotation observability"} {
		ep := filepath.Join(squadDir, "epics", slugFromTitle(epicTitle)+".md")
		if _, err := os.Stat(ep); err != nil {
			t.Errorf("epic file missing for %q: %v", epicTitle, err)
		}
	}

	var specRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM specs WHERE repo_id=? AND name=?`,
		testRepoID, specSlug).Scan(&specRows); err != nil {
		t.Fatalf("count specs: %v", err)
	}
	if specRows != 1 {
		t.Errorf("spec rows = %d want 1", specRows)
	}

	for _, epicTitle := range []string{"core rotation primitives", "rotation observability"} {
		slug := slugFromTitle(epicTitle)
		var n int
		var spec string
		if err := db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(spec),'') FROM epics WHERE repo_id=? AND name=?`,
			testRepoID, slug).Scan(&n, &spec); err != nil {
			t.Fatalf("count epic %s: %v", slug, err)
		}
		if n != 1 {
			t.Errorf("epic rows for %s = %d want 1", slug, n)
		}
		if spec != specSlug {
			t.Errorf("epic %s spec column = %q want %q", slug, spec, specSlug)
		}
	}

	wantEpicForItem := map[string]string{
		"session store dual-read":  slugFromTitle("core rotation primitives"),
		"issuer key roll endpoint": slugFromTitle("core rotation primitives"),
		"rotation metrics":         slugFromTitle("rotation observability"),
	}
	for _, id := range res.ItemIDs {
		var title, parentSpec, epicID, sessionLink sql.NullString
		err := db.QueryRow(
			`SELECT title, COALESCE(parent_spec,''), COALESCE(epic_id,''), COALESCE(intake_session_id,'')
			 FROM items WHERE repo_id=? AND item_id=?`,
			testRepoID, id,
		).Scan(&title, &parentSpec, &epicID, &sessionLink)
		if err != nil {
			t.Fatalf("scan item %s: %v", id, err)
		}
		if parentSpec.String != specSlug {
			t.Errorf("item %s (%s) parent_spec=%q want %q",
				id, title.String, parentSpec.String, specSlug)
		}
		want, ok := wantEpicForItem[title.String]
		if !ok {
			t.Errorf("unexpected item title %q", title.String)
			continue
		}
		if epicID.String != want {
			t.Errorf("item %s (%s) epic_id=%q want %q",
				id, title.String, epicID.String, want)
		}
		if sessionLink.String != sess.ID {
			t.Errorf("item %s intake_session_id=%q want %q",
				id, sessionLink.String, sess.ID)
		}
	}

	var sessStatus, shape string
	if err := db.QueryRow(
		`SELECT status, COALESCE(shape,'') FROM intake_sessions WHERE id=?`,
		sess.ID,
	).Scan(&sessStatus, &shape); err != nil {
		t.Fatalf("session row: %v", err)
	}
	if sessStatus != StatusCommitted {
		t.Errorf("session.status=%q want committed", sessStatus)
	}
	if shape != ShapeSpecEpicItems {
		t.Errorf("session.shape=%q want %q", shape, ShapeSpecEpicItems)
	}
}

func TestCommit_SpecEpicItems_SpecSlugConflict_RollsBackEverything(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	bundle := validSpecEpicItemsBundle()
	specSlug := slugFromTitle(bundle.Spec.Title)

	if _, err := db.Exec(
		`INSERT INTO specs (repo_id, name, title, path, updated_at) VALUES (?, ?, ?, ?, 0)`,
		testRepoID, specSlug, "preexisting", filepath.Join(squadDir, "specs", specSlug+".md"),
	); err != nil {
		t.Fatalf("seed spec conflict: %v", err)
	}

	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, false)
	if err == nil {
		t.Fatalf("commit should fail on spec slug conflict")
	}
	var conflict *IntakeSlugConflict
	if !errors.As(err, &conflict) {
		t.Errorf("err=%v; want *IntakeSlugConflict", err)
	}
	if conflict != nil && (conflict.Kind != "spec" || conflict.Slug != specSlug) {
		t.Errorf("conflict=%+v want spec/%s", conflict, specSlug)
	}

	if entries, _ := os.ReadDir(filepath.Join(squadDir, "epics")); len(entries) > 0 {
		t.Errorf("epics dir should be empty after rollback, got %d entries", len(entries))
	}
	if entries, _ := os.ReadDir(filepath.Join(squadDir, "items")); len(entries) > 0 {
		t.Errorf("items dir should be empty after rollback, got %d entries", len(entries))
	}
	specsEntries, _ := os.ReadDir(filepath.Join(squadDir, "specs"))
	for _, e := range specsEntries {
		// Pre-existing seed put a row but no file; we should not have created
		// any spec markdown file in this squadDir as part of this aborted commit.
		t.Errorf("spec file should not exist after rollback, found %s", e.Name())
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM epics WHERE repo_id=?`, testRepoID).Scan(&n); err != nil {
		t.Fatalf("count epics: %v", err)
	}
	if n != 0 {
		t.Errorf("epics rows after rollback = %d want 0", n)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE repo_id=?`, testRepoID).Scan(&n); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if n != 0 {
		t.Errorf("items rows after rollback = %d want 0", n)
	}

	var sessStatus string
	if err := db.QueryRow(
		`SELECT status FROM intake_sessions WHERE id=?`, sess.ID,
	).Scan(&sessStatus); err != nil {
		t.Fatalf("session row: %v", err)
	}
	if sessStatus != StatusOpen {
		t.Errorf("session.status=%q want open after rollback", sessStatus)
	}
}

// TestCommit_SpecEpicItems_EpicSlugConflict_NoSpecWritten verifies that
// a conflict on any epic slug aborts the commit before any spec or epic
// file is written. The pre-flight check covers all epics up front, so
// the "rollback" here is really "we never started"; the assertion is
// that no side effects landed.
func TestCommit_SpecEpicItems_EpicSlugConflict_NoSpecWritten(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := setupSquadDir(t)
	sess := openSessionForCommit(t, db, squadDir, "agent-1")

	bundle := validSpecEpicItemsBundle()
	conflictingEpicSlug := slugFromTitle(bundle.Epics[1].Title)
	if _, err := db.Exec(
		`INSERT INTO epics (repo_id, name, spec, path, updated_at) VALUES (?, ?, '', ?, 0)`,
		testRepoID, conflictingEpicSlug, filepath.Join(squadDir, "epics", conflictingEpicSlug+".md"),
	); err != nil {
		t.Fatalf("seed epic conflict: %v", err)
	}

	_, err := Commit(ctx, db, squadDir, sess.ID, "agent-1", bundle, false)
	if err == nil {
		t.Fatalf("commit should fail on epic slug conflict")
	}
	if !strings.Contains(err.Error(), conflictingEpicSlug) {
		t.Errorf("err = %v; should mention conflicting epic slug %q", err, conflictingEpicSlug)
	}

	specSlug := slugFromTitle(bundle.Spec.Title)
	if _, err := os.Stat(filepath.Join(squadDir, "specs", specSlug+".md")); err == nil {
		t.Errorf("spec file should not exist after rollback")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM specs WHERE repo_id=?`, testRepoID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("specs rows after rollback = %d want 0", n)
	}
}
