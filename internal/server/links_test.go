package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type linksFixture struct {
	repoRoot string
	squadDir string
}

func newLinksFixture(t *testing.T) (*Server, *linksFixture) {
	t.Helper()
	repoRoot := t.TempDir()
	squadDir := filepath.Join(repoRoot, ".squad")
	for _, sub := range []string{"items", "done"} {
		if err := os.MkdirAll(filepath.Join(squadDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	db := newTestDB(t)
	insertRepo(t, db, testRepoID, repoRoot, "git@github.com:zsiec/squad.git")
	s := New(db, testRepoID, Config{
		RepoID:        testRepoID,
		SquadDir:      squadDir,
		LearningsRoot: repoRoot,
	})
	t.Cleanup(s.Close)
	return s, &linksFixture{repoRoot: repoRoot, squadDir: squadDir}
}

func (f *linksFixture) writeItem(t *testing.T, id, status string) {
	t.Helper()
	dir := "items"
	if status == "done" {
		dir = "done"
	}
	body := fmt.Sprintf(`---
id: %s
title: test
type: bug
priority: P3
area: x
status: %s
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: web
captured_at: 0
accepted_by: web
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---

## Problem
test
`, id, status)
	path := filepath.Join(f.squadDir, dir, id+"-test.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *linksFixture) writePendingPRs(t *testing.T, entries string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.squadDir, "pending-prs.json"), []byte(entries), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedCommit(t *testing.T, s *Server, itemID, sha, subject string, ts int64) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `
		INSERT INTO commits (repo_id, item_id, sha, subject, ts, agent_id)
		VALUES (?, ?, ?, ?, ?, 'agent-a')
	`, testRepoID, itemID, sha, subject, ts); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, s *Server, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound || rec.Body.Len() == 0 {
		return rec.Code, nil
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode %q: %v\nbody=%s", path, err, rec.Body.String())
	}
	return rec.Code, out
}

func TestItemLinks_UnknownItem404(t *testing.T) {
	s, _ := newLinksFixture(t)
	code, _ := getJSON(t, s, "/api/items/UNKNOWN-1/links")
	if code != http.StatusNotFound {
		t.Fatalf("code=%d want 404", code)
	}
}

func TestItemLinks_KnownItemNoPRNoCommits(t *testing.T) {
	s, f := newLinksFixture(t)
	f.writeItem(t, "BUG-101", "done")
	code, body := getJSON(t, s, "/api/items/BUG-101/links")
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%v", code, body)
	}
	if body["pr"] != nil {
		t.Fatalf("pr=%v want nil", body["pr"])
	}
	commits, _ := body["commits"].([]any)
	if len(commits) != 0 {
		t.Fatalf("commits=%v want []", body["commits"])
	}
}

func TestItemLinks_NonGithubOriginShortCircuits(t *testing.T) {
	repoRoot := t.TempDir()
	squadDir := filepath.Join(repoRoot, ".squad")
	for _, sub := range []string{"items", "done"} {
		_ = os.MkdirAll(filepath.Join(squadDir, sub), 0o755)
	}
	db := newTestDB(t)
	insertRepo(t, db, testRepoID, repoRoot, "https://gitlab.com/o/r.git")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: squadDir, LearningsRoot: repoRoot})
	t.Cleanup(s.Close)

	body := `---
id: BUG-202
title: test
type: bug
priority: P3
area: x
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: web
captured_at: 0
accepted_by: web
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---
test
`
	if err := os.WriteFile(filepath.Join(squadDir, "done", "BUG-202-test.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(squadDir, "pending-prs.json"),
		[]byte(`[{"item_id":"BUG-202","branch":"feat/foo","created_at":"2026-04-26T00:00:00Z"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Seed commits anyway — non-github origin must still short-circuit.
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO commits (repo_id, item_id, sha, subject, ts, agent_id)
		VALUES (?, 'BUG-202', 'abc123', 'test commit', 100, 'agent-a')
	`, testRepoID); err != nil {
		t.Fatal(err)
	}

	code, out := getJSON(t, s, "/api/items/BUG-202/links")
	if code != http.StatusOK {
		t.Fatalf("code=%d out=%v", code, out)
	}
	if out["pr"] != nil {
		t.Fatalf("pr=%v want nil for non-github origin", out["pr"])
	}
	commits, _ := out["commits"].([]any)
	if len(commits) != 0 {
		t.Fatalf("commits=%v want [] for non-github origin", out["commits"])
	}
}

func TestItemLinks_PRAndCommitsFromTable(t *testing.T) {
	s, f := newLinksFixture(t)
	f.writeItem(t, "BUG-301", "done")
	f.writePendingPRs(t, `[{"item_id":"BUG-301","branch":"feat/foo","created_at":"2026-04-26T00:00:00Z"}]`)
	seedCommit(t, s, "BUG-301", "1111111111111111111111111111111111111111", "first", 100)
	seedCommit(t, s, "BUG-301", "2222222222222222222222222222222222222222", "second", 200)

	code, out := getJSON(t, s, "/api/items/BUG-301/links")
	if code != http.StatusOK {
		t.Fatalf("code=%d out=%v", code, out)
	}
	pr, _ := out["pr"].(map[string]any)
	if pr == nil {
		t.Fatalf("pr is nil; want compare URL for branch feat/foo")
	}
	if pr["branch"] != "feat/foo" {
		t.Fatalf("pr.branch=%v want feat/foo", pr["branch"])
	}
	if url, _ := pr["url"].(string); url != "https://github.com/zsiec/squad/compare/feat/foo?expand=1" {
		t.Fatalf("pr.url=%q", url)
	}
	commits, _ := out["commits"].([]any)
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2 — %v", len(commits), commits)
	}
	first, _ := commits[0].(map[string]any)
	if first["subject"] != "second" {
		t.Fatalf("commits[0].subject=%v want 'second' (newest first)", first["subject"])
	}
	for _, raw := range commits {
		c, _ := raw.(map[string]any)
		url, _ := c["url"].(string)
		want := "https://github.com/zsiec/squad/commit/" + c["sha"].(string)
		if url != want {
			t.Fatalf("commit url=%q want %q", url, want)
		}
	}
}

func TestItemLinks_CommitsWithoutPR(t *testing.T) {
	s, f := newLinksFixture(t)
	f.writeItem(t, "BUG-401", "done")
	// No pending-prs.json entry — commits should still surface.
	seedCommit(t, s, "BUG-401", "abcdef0123456789abcdef0123456789abcdef01", "solo commit", 50)

	code, out := getJSON(t, s, "/api/items/BUG-401/links")
	if code != http.StatusOK {
		t.Fatalf("code=%d out=%v", code, out)
	}
	if out["pr"] != nil {
		t.Fatalf("pr=%v want nil (no pending-prs entry)", out["pr"])
	}
	commits, _ := out["commits"].([]any)
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1 — %v", len(commits), commits)
	}
}
