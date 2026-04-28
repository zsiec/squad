package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedLearningInRepo writes a minimal approved-gotcha learning under
// <root>/.squad/learnings/gotchas/approved/<slug>.md.
func seedLearningInRepo(t *testing.T, root, slug, title string) {
	t.Helper()
	dir := filepath.Join(root, ".squad", "learnings", "gotchas", "approved")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
id: GOTCHA-` + slug + `
kind: gotcha
slug: ` + slug + `
title: ` + title + `
area: server
paths:
  - internal/server/**
created: 2026-04-28
created_by: agent-test
session: test-session
state: approved
evidence:
  - tests pass
related_items: []
---

# ` + title + `

## Looks like

A test harness needs a parseable gotcha fixture.

## Is

Gotcha frontmatter requires both ` + "`## Looks like`" + ` and ` + "`## Is`" + ` headers.
`
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedLearningsTwoRepos(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	repoA := filepath.Join(tmp, "repoA")
	repoB := filepath.Join(tmp, "repoB")
	seedLearningInRepo(t, repoA, "gotcha-from-a", "From A")
	seedLearningInRepo(t, repoB, "gotcha-from-b", "From B")
	insertRepo(t, db, "repo-A", repoA, "")
	insertRepo(t, db, "repo-B", repoB, "")
	return repoA, repoB
}

func TestHandleLearningsList_WorkspaceModeAggregatesAcrossRepos(t *testing.T) {
	db := newTestDB(t)
	seedLearningsTwoRepos(t, db)

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/learnings", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
	if len(out) != 2 {
		t.Fatalf("workspace-mode learnings should aggregate cross-repo; got %d rows: %v", len(out), out)
	}
	repos := map[string]string{}
	for _, row := range out {
		slug, _ := row["slug"].(string)
		rid, _ := row["repo_id"].(string)
		repos[slug] = rid
	}
	if repos["gotcha-from-a"] != "repo-A" {
		t.Errorf("gotcha-from-a repo_id=%q want repo-A", repos["gotcha-from-a"])
	}
	if repos["gotcha-from-b"] != "repo-B" {
		t.Errorf("gotcha-from-b repo_id=%q want repo-B", repos["gotcha-from-b"])
	}
}

func TestHandleLearningDetail_RepoIDDisambiguates(t *testing.T) {
	db := newTestDB(t)
	tmp := t.TempDir()
	repoA := filepath.Join(tmp, "repoA")
	repoB := filepath.Join(tmp, "repoB")
	// Same slug in both repos to force disambiguation.
	seedLearningInRepo(t, repoA, "shared-slug", "From A")
	seedLearningInRepo(t, repoB, "shared-slug", "From B")
	insertRepo(t, db, "repo-A", repoA, "")
	insertRepo(t, db, "repo-B", repoB, "")

	s := New(db, "", Config{RepoID: ""})
	t.Cleanup(s.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/learnings/shared-slug?repo_id=repo-B", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("?repo_id=repo-B should resolve unambiguously; got %d body=%s",
			rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["title"] != "From B" {
		t.Errorf("?repo_id=repo-B should return repo-B's row; title=%v", out["title"])
	}
	if out["repo_id"] != "repo-B" {
		t.Errorf("response should include repo_id; got %v", out)
	}
}

func TestHandleLearningsList_SingleRepoModeStillScopes(t *testing.T) {
	// Pin that single-repo mode (cfg.RepoID != "") walks cfg.LearningsRoot
	// only and does NOT enumerate the repos table — the workspace branch
	// must stay dormant in single-repo configs.
	db := newTestDB(t)
	tmp := t.TempDir()
	seedLearningInRepo(t, tmp, "scoped", "Single-repo only")
	// A second repo registered in the DB but NOT walked because we're
	// in single-repo mode.
	other := filepath.Join(t.TempDir(), "other")
	seedLearningInRepo(t, other, "other-repo-only", "Should not appear")
	insertRepo(t, db, "repo-other", other, "")

	s := New(db, testRepoID, Config{RepoID: testRepoID, LearningsRoot: tmp})
	t.Cleanup(s.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/learnings", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "scoped") {
		t.Errorf("single-repo mode should include cfg.LearningsRoot's learning: %s", body)
	}
	if strings.Contains(body, "other-repo-only") {
		t.Errorf("single-repo mode must NOT enumerate the repos table: %s", body)
	}
	// Response shape matches specs/epics: repo_id is always emitted, set
	// to cfg.RepoID in single-repo mode.
	if !strings.Contains(body, `"repo_id":"`+testRepoID+`"`) {
		t.Errorf("single-repo response should tag rows with cfg.RepoID; got %s", body)
	}
}
