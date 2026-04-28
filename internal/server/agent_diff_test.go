package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitInWorktree runs `git <args...>` in the given dir and fails the test
// on non-zero exit. Inline helper because the existing internal/scaffold
// test scaffolding doesn't fit this fixture's needs.
func gitInWorktree(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.invalid",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.invalid",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// buildAgentDiffFixture materialises a minimal git repo with two
// branches: `main` (the merge target) and `feature` (a worktree branch
// with one committed file change, one uncommitted edit, and one new
// untracked file). Returns the worktree path the handler should diff.
func buildAgentDiffFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitInWorktree(t, root, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "doomed.txt"), []byte("delete me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInWorktree(t, root, "add", ".")
	gitInWorktree(t, root, "commit", "-q", "-m", "initial")
	gitInWorktree(t, root, "checkout", "-q", "-b", "feature")
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("one\ntwo MODIFIED\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beta.txt"), []byte("brand new file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHandleAgentDiff_ReturnsWorktreeDelta(t *testing.T) {
	worktree := buildAgentDiffFixture(t)
	db := newTestDB(t)
	registerAgent(t, db, "agent-diff-test", "Diff Tester")
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO claims (item_id, repo_id, agent_id, claimed_at, last_touch, intent, long, worktree)
		VALUES ('BUG-DIFF', ?, 'agent-diff-test', 1, 1, 'test', 0, ?)
	`, testRepoID, worktree); err != nil {
		t.Fatalf("insert claim: %v", err)
	}

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: filepath.Join(t.TempDir(), ".squad")})
	t.Cleanup(s.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-diff-test/diff", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		MergeTarget string `json:"merge_target"`
		Files       []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
			Hunks  string `json:"hunks"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, rec.Body.String())
	}
	if body.MergeTarget != "main" {
		t.Errorf("merge_target=%q want main", body.MergeTarget)
	}
	byPath := map[string]struct {
		status string
		hunks  string
	}{}
	for _, f := range body.Files {
		byPath[f.Path] = struct {
			status string
			hunks  string
		}{f.Status, f.Hunks}
	}
	if got, ok := byPath["alpha.txt"]; !ok || got.status != "modified" {
		t.Errorf("alpha.txt: got %v, want modified", got)
	} else if !contains(got.hunks, "two MODIFIED") {
		t.Errorf("alpha.txt hunks missing the modification: %q", got.hunks)
	}
	if got, ok := byPath["doomed.txt"]; !ok || got.status != "deleted" {
		t.Errorf("doomed.txt: got %v, want deleted", got)
	}
	if got, ok := byPath["beta.txt"]; !ok || got.status != "added" {
		t.Errorf("beta.txt: got %v, want added", got)
	} else if !contains(got.hunks, "brand new file") {
		t.Errorf("beta.txt hunks missing the new content: %q", got.hunks)
	}
}

func TestHandleAgentDiff_NoActiveClaim(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-no-claim", "Idle")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: filepath.Join(t.TempDir(), ".squad")})
	t.Cleanup(s.Close)
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-no-claim/diff", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Files []any `json:"files"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Files) != 0 {
		t.Errorf("idle agent should return empty files; got %v", body.Files)
	}
}

// contains is a deliberate inline helper so the test stays readable
// without dragging in strings.Contains via the per-file imports list.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
