package claims

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/store"
)

func TestClaim_FirstCallerWins(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	if err := s.Claim(ctx, "BUG-001", "agent-a", "ship the fix", nil, false); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	err := s.Claim(ctx, "BUG-001", "agent-b", "me too", nil, false)
	if !errors.Is(err, ErrClaimTaken) {
		t.Fatalf("second claim: want ErrClaimTaken, got %v", err)
	}
}

func TestClaim_RaceProducesExactlyOneWinner(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	const racers = 8
	var wg sync.WaitGroup
	wins := make(chan string, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			agent := "agent-" + string(rune('a'+idx))
			if err := s.Claim(ctx, "BUG-002", agent, "race", nil, false); err == nil {
				wins <- agent
			}
		}(i)
	}
	wg.Wait()
	close(wins)
	count := 0
	for range wins {
		count++
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 winner across %d racers, got %d", racers, count)
	}
}

func TestClaim_EmitsBothGlobalAndItemThreadMessages(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()

	if err := s.Claim(ctx, "BUG-003", "agent-a", "intent here", nil, false); err != nil {
		t.Fatal(err)
	}

	var g, i int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE kind='claim' AND thread='global'`).Scan(&g)
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE kind='claim' AND thread='BUG-003'`).Scan(&i)
	if g != 1 || i != 1 {
		t.Fatalf("messages: global=%d item=%d (want 1/1)", g, i)
	}
}

func TestClaim_TouchesPersistedAsActiveRows(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()

	touches := []string{"internal/foo/foo.go", "cmd/squad/bar.go"}
	if err := s.Claim(ctx, "BUG-004", "agent-a", "", touches, false); err != nil {
		t.Fatal(err)
	}

	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM touches WHERE agent_id='agent-a' AND item_id='BUG-004' AND released_at IS NULL`).Scan(&n)
	if n != 2 {
		t.Fatalf("active touches=%d want 2", n)
	}
}

func TestClaim_RefusesWhenBlockerNotInDoneDir(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	_ = os.MkdirAll(itemsDir, 0o755)
	_ = os.MkdirAll(doneDir, 0o755)

	_ = os.WriteFile(filepath.Join(itemsDir, "FEAT-002-foo.md"), []byte(`---
id: FEAT-002
title: foo
status: ready
---
`), 0o644)
	_ = os.WriteFile(filepath.Join(itemsDir, "BUG-070-bar.md"), []byte(`---
id: BUG-070
title: bar
status: ready
blocked-by: [FEAT-002]
---
`), 0o644)

	err := s.Claim(ctx, "BUG-070", "agent-a", "trying", nil, false, ClaimWithPreflight(itemsDir, doneDir))
	if !errors.Is(err, ErrBlockedByOpen) {
		t.Fatalf("want ErrBlockedByOpen, got %v", err)
	}
}

func TestClaim_AllowedWhenBlockersAreInDoneDir(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	_ = os.MkdirAll(itemsDir, 0o755)
	_ = os.MkdirAll(doneDir, 0o755)
	_ = os.WriteFile(filepath.Join(doneDir, "FEAT-002-foo.md"), []byte(`---
id: FEAT-002
title: foo
status: done
---
`), 0o644)
	_ = os.WriteFile(filepath.Join(itemsDir, "BUG-071-bar.md"), []byte(`---
id: BUG-071
title: bar
status: ready
blocked-by: [FEAT-002]
---
`), 0o644)

	if err := s.Claim(ctx, "BUG-071", "agent-a", "go", nil, false, ClaimWithPreflight(itemsDir, doneDir)); err != nil {
		t.Fatalf("claim should succeed: %v", err)
	}
}

func TestClaim_ConflictsWithBlocksOverlap(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	itemsDir := filepath.Join(dir, ".squad", "items")
	doneDir := filepath.Join(dir, ".squad", "done")
	for _, p := range []string{itemsDir, doneDir} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(id string, conflicts []string) {
		yl := ""
		for _, c := range conflicts {
			yl += "  - " + c + "\n"
		}
		b := "---\nid: " + id + "\ntitle: t\ntype: feature\npriority: P1\n" +
			"area: core\nstatus: open\nestimate: 1h\nrisk: low\n" +
			"created: 2026-04-25\nupdated: 2026-04-25\nconflicts_with:\n" + yl + "---\n"
		if err := os.WriteFile(filepath.Join(itemsDir, id+".md"), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("FEAT-700", []string{"a.go", "shared.go"})
	write("FEAT-701", []string{"b.go", "shared.go"})
	write("FEAT-702", []string{"c.go", "d.go"})

	ctx := context.Background()
	w, err := items.Walk(filepath.Join(dir, ".squad"))
	if err != nil {
		t.Fatal(err)
	}
	if err := items.Mirror(ctx, db, "repo-1", w); err != nil {
		t.Fatal(err)
	}
	s := New(db, "repo-1", nil)

	if err := s.Claim(ctx, "FEAT-700", "agent-A", "x", nil, false,
		ClaimWithPreflight(itemsDir, doneDir)); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	err = s.Claim(ctx, "FEAT-701", "agent-B", "x", nil, false,
		ClaimWithPreflight(itemsDir, doneDir))
	if !errors.Is(err, ErrConflictsWithActive) {
		t.Fatalf("expected ErrConflictsWithActive, got %v", err)
	}
	if err := s.Claim(ctx, "FEAT-702", "agent-C", "x", nil, false,
		ClaimWithPreflight(itemsDir, doneDir)); err != nil {
		t.Fatalf("disjoint claim: %v", err)
	}
}
