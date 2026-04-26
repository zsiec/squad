package claims

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlocked_AddsBlockerSectionWhenAbsent(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "BUG-030-thing.md")
	contents := `---
id: BUG-030
title: thing
status: in-progress
created: 2026-04-20
updated: 2026-04-20
---

## Problem
A thing.
`
	_ = os.WriteFile(path, []byte(contents), 0o644)
	_ = s.Claim(ctx, "BUG-030", "agent-a", "", nil, false)

	if err := s.Blocked(ctx, "BUG-030", "agent-a", BlockedOpts{
		Reason:   "waiting on upstream library",
		ItemPath: path,
	}); err != nil {
		t.Fatal(err)
	}

	updated, _ := os.ReadFile(path)
	if !strings.Contains(string(updated), "status: blocked") {
		t.Fatalf("status not updated:\n%s", updated)
	}
	if !strings.Contains(string(updated), "## Blocker") {
		t.Fatalf("Blocker section not appended:\n%s", updated)
	}
	if !strings.Contains(string(updated), "waiting on upstream library") {
		t.Fatalf("blocker reason missing:\n%s", updated)
	}
}

func TestBlocked_PersistsItemRowImmediately(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "BUG-088-persist.md")
	contents := `---
id: BUG-088
title: persist
status: in-progress
created: 2026-04-20
updated: 2026-04-20
---

## Problem
A thing.
`
	_ = os.WriteFile(path, []byte(contents), 0o644)
	_ = s.Claim(ctx, "BUG-088", "agent-a", "", nil, false)

	if err := s.Blocked(ctx, "BUG-088", "agent-a", BlockedOpts{
		Reason:   "waiting on upstream",
		ItemPath: path,
	}); err != nil {
		t.Fatal(err)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM items WHERE repo_id='repo-test' AND item_id='BUG-088'`).Scan(&status); err != nil {
		t.Fatalf("items row missing after Blocked: %v", err)
	}
	if status != "blocked" {
		t.Errorf("status=%q want blocked", status)
	}
}

func TestBlocked_LeavesExistingBlockerSectionAlone(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "BUG-031-thing.md")
	contents := `---
id: BUG-031
title: thing
status: in-progress
created: 2026-04-20
updated: 2026-04-20
---

## Problem
A thing.

## Blocker
prior blocker text the operator already wrote
`
	_ = os.WriteFile(path, []byte(contents), 0o644)
	_ = s.Claim(ctx, "BUG-031", "agent-a", "", nil, false)

	if err := s.Blocked(ctx, "BUG-031", "agent-a", BlockedOpts{
		Reason:   "different reason now",
		ItemPath: path,
	}); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(path)
	if !strings.Contains(string(updated), "prior blocker text the operator already wrote") {
		t.Fatalf("existing Blocker section was overwritten:\n%s", updated)
	}
	if c := strings.Count(string(updated), "\n## Blocker"); c != 1 {
		t.Fatalf("expected exactly 1 Blocker section, got %d:\n%s", c, updated)
	}
}
