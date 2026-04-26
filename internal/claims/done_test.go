package claims

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDone_PostsMessageReleasesAndRecordsOutcome(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	_ = s.Claim(ctx, "BUG-020", "agent-a", "", nil, false)

	if err := s.Done(ctx, "BUG-020", "agent-a", DoneOpts{Summary: "shipped in commit abcdef"}); err != nil {
		t.Fatalf("done: %v", err)
	}

	var msgCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE kind='done' AND thread='BUG-020'`).Scan(&msgCount)
	if msgCount != 1 {
		t.Fatalf("done message count=%d want 1", msgCount)
	}
	var outcome string
	_ = db.QueryRow(`SELECT outcome FROM claim_history WHERE item_id='BUG-020'`).Scan(&outcome)
	if outcome != "done" {
		t.Fatalf("outcome=%q want done", outcome)
	}
}

func TestDone_AtomicWhenReleaseFails(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()

	if err := s.Done(ctx, "BUG-021", "agent-a", DoneOpts{Summary: "x"}); err == nil {
		t.Fatal("expected Done to fail on unclaimed item")
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE thread='BUG-021'`).Scan(&n)
	if n != 0 {
		t.Fatalf("messages leaked despite failure: %d", n)
	}
}

// Regression: Done used to commit the DB tx before the file rewrite. A
// release failure (e.g., claim wasn't ours) left the file moved with no
// way to recover automatically. New ordering (files first, DB second)
// rolls the file back when the DB tx fails.
func TestDone_RollsBackFileMoveWhenDBFails(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(itemsDir, "BUG-099-rollback.md")
	contents := `---
id: BUG-099
title: rollback
type: bug
status: ready
created: 2026-04-20
updated: 2026-04-20
---

## Problem
.
`
	if err := os.WriteFile(itemPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	// No active claim → release will fail → DB tx errors. Verify the
	// file ends up back in items/, not stranded in done/.
	err := s.Done(ctx, "BUG-099", "agent-a", DoneOpts{
		Summary:  "rollback",
		ItemPath: itemPath,
		DoneDir:  doneDir,
	})
	if err == nil {
		t.Fatal("expected Done to fail when no claim exists")
	}
	if _, err := os.Stat(itemPath); err != nil {
		t.Fatalf("item file should be back in items/, got: %v", err)
	}
	stranded := filepath.Join(doneDir, "BUG-099-rollback.md")
	if _, err := os.Stat(stranded); !os.IsNotExist(err) {
		t.Fatalf("file stranded in done/: err=%v", err)
	}
}

func TestDone_PersistsItemRowImmediately(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(itemsDir, "BUG-077-persist.md")
	contents := `---
id: BUG-077
title: persist
type: bug
status: ready
created: 2026-04-20
updated: 2026-04-20
---

## Problem
.
`
	if err := os.WriteFile(itemPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = s.Claim(ctx, "BUG-077", "agent-a", "", nil, false)
	if err := s.Done(ctx, "BUG-077", "agent-a", DoneOpts{
		Summary:  "shipped",
		ItemPath: itemPath,
		DoneDir:  doneDir,
	}); err != nil {
		t.Fatal(err)
	}

	var status string
	var archived int
	var path string
	if err := db.QueryRow(`SELECT status, archived, path FROM items WHERE repo_id='repo-test' AND item_id='BUG-077'`).Scan(&status, &archived, &path); err != nil {
		t.Fatalf("items row missing after Done: %v", err)
	}
	if status != "done" {
		t.Errorf("status=%q want done", status)
	}
	if archived != 1 {
		t.Errorf("archived=%d want 1", archived)
	}
	wantPath := filepath.Join(doneDir, "BUG-077-persist.md")
	if path != wantPath {
		t.Errorf("path=%q want %q", path, wantPath)
	}
}

func TestDone_RewritesFrontmatterAndMovesToDoneDir(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	itemPath := filepath.Join(itemsDir, "BUG-022-thing.md")
	contents := `---
id: BUG-022
title: thing
type: bug
status: ready
created: 2026-04-20
updated: 2026-04-20
---

## Problem
A thing.
`
	if err := os.WriteFile(itemPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = s.Claim(ctx, "BUG-022", "agent-a", "", nil, false)
	if err := s.Done(ctx, "BUG-022", "agent-a", DoneOpts{
		Summary:  "done!",
		ItemPath: itemPath,
		DoneDir:  doneDir,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(itemPath); !os.IsNotExist(err) {
		t.Fatalf("item file still at original path: err=%v", err)
	}
	movedPath := filepath.Join(doneDir, "BUG-022-thing.md")
	moved, err := os.ReadFile(movedPath)
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if !strings.Contains(string(moved), "status: done") {
		t.Fatalf("moved file missing status: done\n%s", moved)
	}
	if !strings.Contains(string(moved), "## Problem") {
		t.Fatalf("moved file lost body content")
	}
}
