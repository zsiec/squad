package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDone_CleansUpWorktreeAfterRelease(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	writeMinimalItem(t, env.ItemsDir, "BUG-500")

	res, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-500",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true, RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if _, err := os.Stat(res.WorktreePath); err != nil {
		t.Fatalf("provisioned dir missing: %v", err)
	}

	if _, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-500",
		Summary:  "shipped",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	}); err != nil {
		t.Fatalf("Done: %v", err)
	}

	if _, err := os.Stat(res.WorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still present after done: err=%v", err)
	}
}

func TestDone_WithoutWorktreeIsNoOp(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-501")
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-501",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	res, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-501",
		Summary:  "shipped",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if res.WorktreeCleanupWarning != "" {
		t.Errorf("unexpected warning: %s", res.WorktreeCleanupWarning)
	}
}

func TestDone_WorktreeCleanupFailureSurfacesAsWarning(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)
	writeMinimalItem(t, env.ItemsDir, "BUG-502")

	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-502",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true, RepoRoot: env.Root,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// Point Done at a non-git directory so worktree.Cleanup fails. Done
	// itself must still succeed: the warning is surfaced but the claim
	// is released and the file is moved to done/.
	bogus := t.TempDir()

	res, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-502",
		Summary:  "shipped",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: bogus,
	})
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if res.WorktreeCleanupWarning == "" {
		t.Error("expected WorktreeCleanupWarning when RepoRoot is not a git repo")
	}

	var n int
	_ = env.DB.QueryRow(`SELECT COUNT(*) FROM claims WHERE repo_id=? AND item_id=?`,
		env.RepoID, "BUG-502").Scan(&n)
	if n != 0 {
		t.Errorf("claim should be released despite cleanup warning, count=%d", n)
	}
	if _, err := os.Stat(filepath.Join(env.DoneDir, "BUG-502.md")); err != nil {
		t.Errorf("item file should be in done/, err=%v", err)
	}
}
