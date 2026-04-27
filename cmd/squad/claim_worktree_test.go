package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitFixtureCommit adds a single commit on `main` in dir so subsequent
// worktree provisions have a base to fork from. setupSquadRepo gives us a
// git repo with no commits — every claim --worktree test needs at least
// one before exercising worktree.Provision.
func gitFixtureCommit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
		{"add", "."},
		{"commit", "-q", "-m", "init"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestClaim_WithWorktreeProvisionsAndRecordsPath(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	writeMinimalItem(t, env.ItemsDir, "BUG-400")

	res, err := Claim(context.Background(), ClaimArgs{
		DB:       env.DB,
		RepoID:   env.RepoID,
		AgentID:  env.AgentID,
		ItemID:   "BUG-400",
		Intent:   "isolated",
		ItemsDir: env.ItemsDir,
		DoneDir:  env.DoneDir,
		Worktree: true,
		RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if res.WorktreePath == "" {
		t.Fatal("WorktreePath empty after --worktree claim")
	}
	if _, err := os.Stat(res.WorktreePath); err != nil {
		t.Errorf("worktree dir missing: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(res.WorktreePath), ".squad/worktrees/"+env.AgentID+"-BUG-400") {
		t.Errorf("worktree path %q does not match convention", res.WorktreePath)
	}

	var stored string
	if err := env.DB.QueryRow(
		`SELECT worktree FROM claims WHERE repo_id=? AND item_id=?`,
		env.RepoID, "BUG-400",
	).Scan(&stored); err != nil {
		t.Fatalf("read worktree column: %v", err)
	}
	if stored != res.WorktreePath {
		t.Errorf("claims.worktree=%q want %q", stored, res.WorktreePath)
	}
}

func TestClaim_WithoutWorktreeLeavesColumnEmpty(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-401")

	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-401",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	var stored string
	_ = env.DB.QueryRow(
		`SELECT worktree FROM claims WHERE repo_id=? AND item_id=?`,
		env.RepoID, "BUG-401",
	).Scan(&stored)
	if stored != "" {
		t.Errorf("worktree column should be empty without --worktree, got %q", stored)
	}
}

func TestClaim_WorktreeFailureLeavesNoClaimRow(t *testing.T) {
	env := newTestEnv(t)
	// No commit → git worktree add against the unborn `main` branch fails.
	writeMinimalItem(t, env.ItemsDir, "BUG-402")

	_, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-402",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true,
		RepoRoot: env.Root,
	})
	if err == nil {
		t.Fatal("expected provision failure on commitless repo")
	}
	var n int
	_ = env.DB.QueryRow(
		`SELECT COUNT(*) FROM claims WHERE repo_id=? AND item_id=?`,
		env.RepoID, "BUG-402",
	).Scan(&n)
	if n != 0 {
		t.Errorf("claim row leaked despite provision failure: count=%d", n)
	}
	if _, statErr := os.Stat(filepath.Join(env.Root, ".squad", "worktrees")); !os.IsNotExist(statErr) {
		entries, _ := os.ReadDir(filepath.Join(env.Root, ".squad", "worktrees"))
		if len(entries) != 0 {
			t.Errorf(".squad/worktrees should be empty on failure, got %d entries", len(entries))
		}
	}
}

