package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitConfigUser sets the test repo's commit identity. Without it the
// worktree-branch commits made inside these tests fail with
// "Author identity unknown". gitFixtureCommit in the existing
// claim_worktree_test.go already does this; we re-use the helper but
// expose it as a package-test fixture so done_worktree_fold_test.go
// can run without the claim test having to be loaded first.
func gitConfigUser(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// commitOnBranch makes a single commit on the named branch in dir,
// adding `relPath` with `body`. Switches branches as a side effect;
// callers should restore the branch they care about.
func commitOnBranch(t *testing.T, dir, branch, relPath, body, message string) {
	t.Helper()
	if out, err := runGit(dir, "switch", branch); err != nil {
		t.Fatalf("switch %s: %v: %s", branch, err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, relPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runGit(dir, "add", relPath); err != nil {
		t.Fatalf("add: %v: %s", err, out)
	}
	if out, err := runGit(dir, "commit", "-m", message); err != nil {
		t.Fatalf("commit: %v: %s", err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(out)
}

// branchExists returns true iff `git rev-parse --verify <branch>`
// succeeds in dir.
func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	if _, err := runGit(dir, "rev-parse", "--verify", "refs/heads/"+branch); err != nil {
		return false
	}
	return true
}

// writeMinimalCapturedItem writes a minimal item file that satisfies
// the captured→open→done lifecycle for these tests. Mirrors the
// fixture in claim_worktree_test.go but covers `status: open` so the
// claim store accepts the claim.
func writeMinimalCapturedItem(t *testing.T, itemsDir, id string) {
	t.Helper()
	body := fmt.Sprintf(`---
id: %s
title: a sufficiently long title for ready
type: feature
priority: P2
area: core
status: open
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
---

## Acceptance criteria
- [ ] verbatim test item
`, id)
	if err := os.WriteFile(filepath.Join(itemsDir, id+"-x.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDone_FoldsWorktreeBranchOnFastForward is the happy-path
// contract: claim with worktree → commit on the per-claim branch →
// squad done → main contains the commit, the per-claim branch is
// deleted, the worktree dir is gone.
func TestDone_FoldsWorktreeBranchOnFastForward(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	gitConfigUser(t, env.Root)
	writeMinimalCapturedItem(t, env.ItemsDir, "FEAT-900")

	claimRes, err := Claim(context.Background(), ClaimArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "FEAT-900",
		Intent:   "fold smoke",
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
		Worktree: true,
		RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if claimRes.WorktreePath == "" {
		t.Fatal("expected worktree path")
	}

	// Make a commit on the per-claim branch via the worktree.
	wantFile := "fold-marker.txt"
	if err := os.WriteFile(filepath.Join(claimRes.WorktreePath, wantFile), []byte("from-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runGit(claimRes.WorktreePath, "add", wantFile); err != nil {
		t.Fatalf("add in worktree: %v: %s", err, out)
	}
	if out, err := runGit(claimRes.WorktreePath, "commit", "-m", "test marker"); err != nil {
		t.Fatalf("commit in worktree: %v: %s", err, out)
	}
	commitSHA := gitOutput(t, claimRes.WorktreePath, "rev-parse", "HEAD")

	// squad done from the parent repo with main checked out.
	doneArgs := DoneArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "FEAT-900",
		Summary:  "test fold",
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
		RepoRoot: env.Root,
		Force:    true, // bypass evidence gate for the test
	}
	res, derr := Done(context.Background(), doneArgs)
	if derr != nil {
		t.Fatalf("Done: %v", derr)
	}
	if res.WorktreeCleanupWarning != "" {
		t.Errorf("unexpected cleanup warning: %s", res.WorktreeCleanupWarning)
	}

	// AC#1: target branch (main) now contains the worktree's commit.
	mainSHA := gitOutput(t, env.Root, "rev-parse", "main")
	if mainSHA != commitSHA {
		t.Errorf("main sha = %s; want %s (worktree's commit folded onto main)", mainSHA, commitSHA)
	}

	// AC#2: per-claim branch deleted.
	branch := "squad/FEAT-900-" + env.AgentID
	if branchExists(t, env.Root, branch) {
		t.Errorf("branch %s still exists after squad done", branch)
	}

	// Worktree directory gone.
	if _, err := os.Stat(claimRes.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still present at %s; stat err=%v", claimRes.WorktreePath, err)
	}
}

// TestDone_NoOpWhenNoWorktreeBranch covers a non-worktree claim:
// squad done must not error and must not invent a branch to fold.
func TestDone_NoOpWhenNoWorktreeBranch(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	gitConfigUser(t, env.Root)
	writeMinimalCapturedItem(t, env.ItemsDir, "FEAT-901")

	if _, err := Claim(context.Background(), ClaimArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "FEAT-901",
		Intent:   "no worktree",
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
		RepoRoot: env.Root,
		// Worktree: false — no worktree provisioned
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	preMain := gitOutput(t, env.Root, "rev-parse", "main")

	if _, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-901", Summary: "no fold",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root, Force: true,
	}); err != nil {
		t.Fatalf("Done (no worktree): %v", err)
	}

	if got := gitOutput(t, env.Root, "rev-parse", "main"); got != preMain {
		t.Errorf("main moved without a worktree to fold: was %s now %s", preMain, got)
	}
}

// TestDone_FoldsWorktreeBranchWithMergeCommitOnDivergence covers
// the no-ff fallback: main has advanced since the worktree was
// claimed, so a fast-forward fails and squad done falls back to a
// merge commit. The worktree's commit is reachable from main, the
// per-claim branch is deleted, and the worktree dir is gone.
func TestDone_FoldsWorktreeBranchWithMergeCommitOnDivergence(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	gitConfigUser(t, env.Root)
	writeMinimalCapturedItem(t, env.ItemsDir, "FEAT-902")

	claimRes, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-902", Intent: "diverge", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true, RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// Commit on the worktree branch.
	worktreeFile := "branch-marker.txt"
	if err := os.WriteFile(filepath.Join(claimRes.WorktreePath, worktreeFile), []byte("branch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runGit(claimRes.WorktreePath, "add", worktreeFile); err != nil {
		t.Fatalf("add: %v: %s", err, out)
	}
	if out, err := runGit(claimRes.WorktreePath, "commit", "-m", "branch commit"); err != nil {
		t.Fatalf("commit: %v: %s", err, out)
	}
	branchSHA := gitOutput(t, claimRes.WorktreePath, "rev-parse", "HEAD")

	// Advance main with an unrelated commit so FF can't apply.
	commitOnBranch(t, env.Root, "main", "main-marker.txt", "main\n", "advance main")
	mainBeforeMerge := gitOutput(t, env.Root, "rev-parse", "main")

	if _, derr := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-902", Summary: "diverge fold",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root, Force: true,
	}); derr != nil {
		t.Fatalf("Done: %v", derr)
	}

	// Main is now a merge commit (two parents). Worktree's commit is
	// reachable from main.
	headSHA := gitOutput(t, env.Root, "rev-parse", "main")
	if headSHA == mainBeforeMerge {
		t.Errorf("main did not advance — fold did not happen on divergence")
	}
	parents := strings.Fields(gitOutput(t, env.Root, "rev-list", "--parents", "-n", "1", "main"))
	if len(parents) != 3 { // <merge-sha> <parent1> <parent2>
		t.Errorf("expected merge commit with 2 parents on main; got parents=%v", parents)
	}
	if _, err := runGit(env.Root, "merge-base", "--is-ancestor", branchSHA, "main"); err != nil {
		t.Errorf("worktree's commit %s not reachable from main: %v", branchSHA, err)
	}

	branch := "squad/FEAT-902-" + env.AgentID
	if branchExists(t, env.Root, branch) {
		t.Errorf("branch %s still exists after divergence fold", branch)
	}
	if _, err := os.Stat(claimRes.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still present after divergence fold")
	}
}

// TestDone_LeavesItemAndBranchOnConflict covers AC#3: a real merge
// conflict between the worktree branch and main must not silently
// archive the item. squad done returns an error, the per-claim
// branch survives, and the item file stays in `.squad/items/` (not
// moved to .squad/done/).
func TestDone_LeavesItemAndBranchOnConflict(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	gitConfigUser(t, env.Root)
	writeMinimalCapturedItem(t, env.ItemsDir, "FEAT-903")

	// Seed a file on main so both sides can edit it differently.
	commitOnBranch(t, env.Root, "main", "shared.txt", "original\n", "seed shared")

	claimRes, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-903", Intent: "conflict", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true, RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	// Edit shared.txt in the worktree.
	if err := os.WriteFile(filepath.Join(claimRes.WorktreePath, "shared.txt"), []byte("from-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runGit(claimRes.WorktreePath, "commit", "-am", "edit shared in worktree"); err != nil {
		t.Fatalf("worktree commit: %v: %s", err, out)
	}
	// Edit shared.txt on main differently — guaranteed conflict.
	commitOnBranch(t, env.Root, "main", "shared.txt", "from-main\n", "edit shared on main")

	itemPath := filepath.Join(env.ItemsDir, "FEAT-903-x.md")
	if _, err := os.Stat(itemPath); err != nil {
		t.Fatalf("setup: item file missing: %v", err)
	}

	_, derr := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "FEAT-903", Summary: "should fail",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root, Force: true,
	})
	if derr == nil {
		t.Fatal("Done must error on merge conflict; got nil")
	}
	if !strings.Contains(strings.ToLower(derr.Error()), "conflict") &&
		!strings.Contains(strings.ToLower(derr.Error()), "merge") {
		t.Errorf("error should explain the conflict; got %v", derr)
	}

	branch := "squad/FEAT-903-" + env.AgentID
	if !branchExists(t, env.Root, branch) {
		t.Errorf("per-claim branch %s deleted on conflict; should be preserved for manual fix", branch)
	}

	// Item file must NOT have moved to done/ — fold runs before
	// claims.Done so a conflict bails out before any archive happens.
	if _, err := os.Stat(itemPath); err != nil {
		t.Errorf("item file moved out of .squad/items/ on conflict; should be preserved for re-run: stat err=%v", err)
	}
	doneFile := filepath.Join(env.DoneDir, "FEAT-903-x.md")
	if _, err := os.Stat(doneFile); !os.IsNotExist(err) {
		t.Errorf("item file appeared in .squad/done/ on conflict; AC says it must stay in items/: stat err=%v", err)
	}

	// Claim must still be held — squad done can be re-run after the
	// conflict is resolved.
	var claimCount int
	if err := env.DB.QueryRow(
		`SELECT COUNT(*) FROM claims WHERE repo_id=? AND item_id=? AND agent_id=?`,
		env.RepoID, "FEAT-903", env.AgentID).Scan(&claimCount); err != nil {
		t.Fatalf("count claims: %v", err)
	}
	if claimCount != 1 {
		t.Errorf("claim released on conflict; should be held for re-run, got %d rows", claimCount)
	}
}
