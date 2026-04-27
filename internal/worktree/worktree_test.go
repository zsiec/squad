package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvision_CreatesWorktreeAndBranch(t *testing.T) {
	root := newGitFixture(t)

	path, branch, err := Provision(root, "main", "FEAT-100", "agent-a")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if branch != "squad/FEAT-100-agent-a" {
		t.Errorf("branch=%q want squad/FEAT-100-agent-a", branch)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree dir missing: %v", err)
	}
	if !strings.Contains(path, ".squad/worktrees/agent-a-FEAT-100") {
		t.Errorf("path=%q does not match convention", path)
	}

	out, err := gitOut(t, root, "branch", "--list", "squad/FEAT-100-agent-a")
	if err != nil {
		t.Fatalf("branch list: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("branch squad/FEAT-100-agent-a not created")
	}
}

func TestProvision_IdempotentReturnsErrExists(t *testing.T) {
	root := newGitFixture(t)

	first, _, err := Provision(root, "main", "FEAT-101", "agent-b")
	if err != nil {
		t.Fatalf("first provision: %v", err)
	}

	second, branch, err := Provision(root, "main", "FEAT-101", "agent-b")
	if !errors.Is(err, ErrExists) {
		t.Fatalf("re-provision: want ErrExists, got %v", err)
	}
	if !pathsEqual(first, second) {
		t.Errorf("re-provision returned %q, want %q", second, first)
	}
	if branch != "squad/FEAT-101-agent-b" {
		t.Errorf("branch=%q", branch)
	}
}

func TestProvision_NotGitRepo(t *testing.T) {
	root := t.TempDir()
	_, _, err := Provision(root, "main", "FEAT-102", "agent-c")
	if !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("want ErrNotGitRepo, got %v", err)
	}
}

func TestCleanup_RemovesDirAndDeletesBranchWhenNoCommits(t *testing.T) {
	root := newGitFixture(t)

	path, branch, err := Provision(root, "main", "FEAT-103", "agent-d")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if err := Cleanup(root, path); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists: err=%v", err)
	}
	out, _ := gitOut(t, root, "branch", "--list", branch)
	if strings.TrimSpace(out) != "" {
		t.Errorf("branch %s should have been deleted, got: %q", branch, out)
	}
}

func TestCleanup_PreservesBranchWhenCommitsLanded(t *testing.T) {
	root := newGitFixture(t)

	path, branch, err := Provision(root, "main", "FEAT-104", "agent-e")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	scratch := filepath.Join(path, "claim.txt")
	if err := os.WriteFile(scratch, []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, path, "add", ".")
	gitMust(t, path, "commit", "-m", "claim work")

	if err := Cleanup(root, path); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	out, err := gitOut(t, root, "branch", "--list", branch)
	if err != nil {
		t.Fatalf("branch list: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("branch %s should be preserved when commits landed", branch)
	}
}

// Regression for finding 1: a repo whose default branch is neither `main`
// nor `master` must still see no-commits squad/ branches deleted on
// Cleanup. Provision now persists the base via `branch.<n>.squadBase`,
// which is what makes this work without hardcoding branch names.
func TestCleanup_DeletesBranchOnNonMainDefault(t *testing.T) {
	root := t.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	gitMust(t, root, "init", "-q", "-b", "develop")
	gitMust(t, root, "config", "user.email", "test@example.com")
	gitMust(t, root, "config", "user.name", "Test")
	gitMust(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, root, "add", ".")
	gitMust(t, root, "commit", "-q", "-m", "init")

	path, branch, err := Provision(root, "develop", "FEAT-110", "agent-x")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	if err := Cleanup(root, path); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	out, _ := gitOut(t, root, "branch", "--list", branch)
	if strings.TrimSpace(out) != "" {
		t.Errorf("branch %s should have been deleted on non-main default, got: %q", branch, out)
	}
}

func TestCleanup_MissingPathIsNoError(t *testing.T) {
	root := newGitFixture(t)
	if err := Cleanup(root, filepath.Join(root, ".squad", "worktrees", "agent-z-FEAT-999")); err != nil {
		t.Fatalf("cleanup of missing path should be tolerated: %v", err)
	}
}

func TestList_IncludesProvisioned(t *testing.T) {
	root := newGitFixture(t)

	path, _, err := Provision(root, "main", "FEAT-105", "agent-f")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	list, err := List(root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 worktrees, got %d: %+v", len(list), list)
	}
	var found bool
	for _, w := range list {
		if pathsEqual(w.Path, path) {
			found = true
			if w.Branch != "squad/FEAT-105-agent-f" {
				t.Errorf("entry branch=%q", w.Branch)
			}
			break
		}
	}
	if !found {
		t.Errorf("provisioned path %q not in list: %+v", path, list)
	}
}

func TestParseWorktreeList_HandlesPorcelain(t *testing.T) {
	in := "worktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree /repo/.squad/worktrees/agent-a-FEAT-1\nHEAD def456\nbranch refs/heads/squad/FEAT-1-agent-a\n\n"
	got := parseWorktreeList(in)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[1].Branch != "squad/FEAT-1-agent-a" {
		t.Errorf("got[1].Branch=%q", got[1].Branch)
	}
	if got[0].HEAD != "abc123" {
		t.Errorf("got[0].HEAD=%q", got[0].HEAD)
	}
}

// newGitFixture initialises a git repo with one commit on the `main`
// branch and returns its absolute, symlink-resolved path. Every test gets
// a fresh repo via t.TempDir, so worktree state never bleeds between tests.
func newGitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	gitMust(t, dir, "init", "-q", "-b", "main")
	gitMust(t, dir, "config", "user.email", "test@example.com")
	gitMust(t, dir, "config", "user.name", "Test")
	gitMust(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, dir, "add", ".")
	gitMust(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func gitMust(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
