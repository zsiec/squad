package intake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSession_OpenRefineRejectsItemMismatchOnResume(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := writeItem(t, "items", "FEAT-100", "captured", "first", "## Problem\nthing one\n")
	addCapturedItem(t, squadDir, "FEAT-200", "second", "## Problem\nthing two\n")

	if _, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		IdeaSeed: "first", RefineItemID: "FEAT-100", SquadDir: squadDir,
	}); err != nil {
		t.Fatalf("open #1: %v", err)
	}

	_, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		IdeaSeed: "second", RefineItemID: "FEAT-200", SquadDir: squadDir,
	})
	if !errors.Is(err, ErrIntakeRefineItemMismatch) {
		t.Fatalf("got %v; want ErrIntakeRefineItemMismatch", err)
	}
}

// addCapturedItem writes a second captured item into an existing
// squadDir (writeItem creates a fresh TempDir per call).
func addCapturedItem(t *testing.T, squadDir, id, title, body string) {
	t.Helper()
	frontmatter := "---\n" +
		"id: " + id + "\n" +
		"title: " + title + "\n" +
		"type: feature\n" +
		"priority: P2\n" +
		"area: auth\n" +
		"status: captured\n" +
		"estimate: 1h\n" +
		"risk: low\n" +
		"created: 2026-04-26\n" +
		"updated: 2026-04-26\n" +
		"captured_by: agent-9f3a\n" +
		"captured_at: 1714150000\n" +
		"---\n\n"
	if err := os.WriteFile(filepath.Join(squadDir, "items", id+".md"), []byte(frontmatter+body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSession_OpenSameRefineItemResumesCleanly(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := writeItem(t, "items", "FEAT-100", "captured", "first", "## Problem\nthing one\n")

	first, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		IdeaSeed: "first", RefineItemID: "FEAT-100", SquadDir: squadDir,
	})
	if err != nil {
		t.Fatalf("open #1: %v", err)
	}

	second, snap, resumed, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		IdeaSeed: "first", RefineItemID: "FEAT-100", SquadDir: squadDir,
	})
	if err != nil {
		t.Fatalf("open #2 (same item): %v", err)
	}
	if !resumed {
		t.Errorf("expected resumed=true on second matching open")
	}
	if second.ID != first.ID {
		t.Errorf("resumed session id mismatch: %q vs %q", second.ID, first.ID)
	}
	if snap.ID != "FEAT-100" {
		t.Errorf("resumed snapshot id = %q; want FEAT-100", snap.ID)
	}
}

func TestSession_OpenModeMismatchOnResume(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	squadDir := writeItem(t, "items", "FEAT-100", "captured", "first", "## Problem\nthing\n")

	if _, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeNew, IdeaSeed: "fresh idea",
	}); err != nil {
		t.Fatalf("open new: %v", err)
	}

	_, _, _, err := Open(ctx, db, OpenParams{
		RepoID: "repo-a", AgentID: "agent-1", Mode: ModeRefine,
		RefineItemID: "FEAT-100", SquadDir: squadDir,
	})
	if !errors.Is(err, ErrIntakeRefineItemMismatch) {
		t.Fatalf("got %v; want ErrIntakeRefineItemMismatch (existing ModeNew vs caller's refine FEAT-100)", err)
	}
}
