package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// linksFixture builds a self-contained world: temp dir as repoRoot, a real
// git repo inside it with deterministic commits, a `.squad/` directory, a
// repos row pinned to that root path, optional touches and pending-prs.
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

func (f *linksFixture) initGit(t *testing.T) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", f.repoRoot}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")
}

func (f *linksFixture) commit(t *testing.T, file, subject string) {
	t.Helper()
	if file != "" {
		w := exec.Command("sh", "-c", fmt.Sprintf("echo %q >> %s", subject, file))
		w.Dir = f.repoRoot
		if out, err := w.CombinedOutput(); err != nil {
			t.Fatalf("write: %v\n%s", err, out)
		}
		add := exec.Command("git", "-C", f.repoRoot, "add", file)
		if out, err := add.CombinedOutput(); err != nil {
			t.Fatalf("add: %v\n%s", err, out)
		}
	}
	c := exec.Command("git", "-C", f.repoRoot, "commit", "--allow-empty", "-m", subject)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
}

func (f *linksFixture) writeItem(t *testing.T, id, status string, acceptedAt int64) {
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
accepted_at: %d
references: []
relates-to: []
blocked-by: []
---

## Problem
test
`, id, status, acceptedAt)
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

func TestItemLinks_KnownItemNoBranchNoTouches(t *testing.T) {
	s, f := newLinksFixture(t)
	f.writeItem(t, "BUG-101", "done", 0)
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

	// Write item directly so we don't reuse newLinksFixture's GitHub origin.
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

	// Even with a pending entry, a non-github origin returns no links.
	if err := os.WriteFile(filepath.Join(squadDir, "pending-prs.json"),
		[]byte(`[{"item_id":"BUG-202","branch":"feat/foo","created_at":"2026-04-26T00:00:00Z"}]`), 0o644); err != nil {
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

func TestItemLinks_FullPRAndCommits(t *testing.T) {
	s, f := newLinksFixture(t)
	f.initGit(t)
	f.commit(t, "before.go", "before claim window")
	since := time.Now()
	time.Sleep(1100 * time.Millisecond) // git --since/--until is second-precision
	f.commit(t, "x.go", "in window touched x")
	f.commit(t, "y.go", "in window touched y")
	f.commit(t, "z.go", "in window touched z (no touch row)")

	f.writeItem(t, "BUG-301", "done", since.Unix())
	f.writePendingPRs(t, `[{"item_id":"BUG-301","branch":"main","created_at":"2026-04-26T00:00:00Z"}]`)

	// Insert touches for x.go and y.go only.
	for _, p := range []string{"x.go", "y.go"} {
		if _, err := s.db.ExecContext(context.Background(), `
			INSERT INTO touches (repo_id, agent_id, item_id, path, started_at, released_at)
			VALUES (?, 'agent-a', 'BUG-301', ?, 0, 0)
		`, testRepoID, p); err != nil {
			t.Fatal(err)
		}
	}

	code, out := getJSON(t, s, "/api/items/BUG-301/links")
	if code != http.StatusOK {
		t.Fatalf("code=%d out=%v", code, out)
	}

	pr, _ := out["pr"].(map[string]any)
	if pr == nil {
		t.Fatalf("pr is nil; want pr with branch=main")
	}
	if pr["branch"] != "main" {
		t.Fatalf("pr.branch=%v want main", pr["branch"])
	}
	if pr["url"] == nil {
		t.Fatalf("pr.url is nil; want compare URL")
	}

	commits, _ := out["commits"].([]any)
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2 (x.go and y.go) — %v", len(commits), commits)
	}
	for _, raw := range commits {
		c, _ := raw.(map[string]any)
		if c["sha"] == nil || c["subject"] == nil || c["url"] == nil {
			t.Fatalf("commit missing fields: %+v", c)
		}
		url, _ := c["url"].(string)
		if !startsWith(url, "https://github.com/zsiec/squad/commit/") {
			t.Fatalf("commit url=%q does not start with expected base", url)
		}
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
