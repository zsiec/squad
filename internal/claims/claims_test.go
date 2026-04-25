package claims

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
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
