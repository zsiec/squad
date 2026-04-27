package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/worktree"
)

func TestDoctor_WorktreeOrphanFlagsUnknownDirs(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)

	// One active claim with a real worktree → must NOT show up as orphan.
	writeMinimalItem(t, env.ItemsDir, "BUG-600")
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-600",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true, RepoRoot: env.Root,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// Provision a second worktree but DON'T record a claim row — this
	// simulates a crashed claim or a manual `git worktree add`.
	orphanPath, _, err := worktree.Provision(env.Root, "main", "BUG-601", "agent-ghost")
	if err != nil {
		t.Fatalf("provision orphan: %v", err)
	}

	res, err := Doctor(context.Background(), DoctorArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}

	var orphans []DoctorFinding
	for _, f := range res.Findings {
		if f.Code == "worktree_orphan" {
			orphans = append(orphans, f)
		}
	}
	if len(orphans) != 1 {
		t.Fatalf("worktree_orphan findings=%d, want 1: %+v", len(orphans), orphans)
	}
	abs, _ := filepath.Abs(orphanPath)
	if !strings.Contains(orphans[0].Message, abs) {
		t.Errorf("finding does not name orphan path %q: %q", abs, orphans[0].Message)
	}
}

func TestDoctor_WorktreeOrphanCleanWhenAllMatched(t *testing.T) {
	env := newTestEnv(t)
	gitFixtureCommit(t, env.Root)

	writeMinimalItem(t, env.ItemsDir, "BUG-602")
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID:   "BUG-602",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		Worktree: true, RepoRoot: env.Root,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	res, err := Doctor(context.Background(), DoctorArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	for _, f := range res.Findings {
		if f.Code == "worktree_orphan" {
			t.Errorf("unexpected worktree_orphan finding: %+v", f)
		}
	}
}

func TestDoctor_WorktreeOrphanQuietWhenDirAbsent(t *testing.T) {
	env := newTestEnv(t)
	if _, err := os.Stat(filepath.Join(env.Root, ".squad", "worktrees")); !os.IsNotExist(err) {
		t.Skipf("test assumes .squad/worktrees absent on init, got err=%v", err)
	}
	res, err := Doctor(context.Background(), DoctorArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	for _, f := range res.Findings {
		if f.Code == "worktree_orphan" {
			t.Errorf("orphan check should be quiet when worktrees dir absent: %+v", f)
		}
	}
}
