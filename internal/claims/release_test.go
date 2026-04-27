package claims

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRelease_MovesActiveClaimToHistory(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	if err := s.Claim(ctx, "BUG-010", "agent-a", "", nil, false); err != nil {
		t.Fatal(err)
	}

	if err := s.Release(ctx, "BUG-010", "agent-a", "released"); err != nil {
		t.Fatalf("release: %v", err)
	}

	var live int
	_ = db.QueryRow(`SELECT COUNT(*) FROM claims WHERE item_id='BUG-010'`).Scan(&live)
	if live != 0 {
		t.Fatalf("claim still active, count=%d", live)
	}
	var outcome string
	_ = db.QueryRow(`SELECT outcome FROM claim_history WHERE item_id='BUG-010'`).Scan(&outcome)
	if outcome != "released" {
		t.Fatalf("outcome=%q want released", outcome)
	}
}

func TestRelease_RejectsForeignClaim(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	_ = s.Claim(ctx, "BUG-011", "agent-a", "", nil, false)

	err := s.Release(ctx, "BUG-011", "agent-b", "released")
	if !errors.Is(err, ErrNotYours) {
		t.Fatalf("want ErrNotYours, got %v", err)
	}
}

func TestRelease_RejectsUnclaimedItem(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	err := s.Release(ctx, "NEVER-EXISTED", "agent-a", "released")
	if !errors.Is(err, ErrNotClaimed) {
		t.Fatalf("want ErrNotClaimed, got %v", err)
	}
}

func TestRelease_MarksAllItemTouchesReleased(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	_ = s.Claim(ctx, "BUG-012", "agent-a", "", []string{"internal/x.go", "internal/y.go"}, false)

	if err := s.Release(ctx, "BUG-012", "agent-a", "released"); err != nil {
		t.Fatal(err)
	}
	var active int
	_ = db.QueryRow(`SELECT COUNT(*) FROM touches WHERE item_id='BUG-012' AND released_at IS NULL`).Scan(&active)
	if active != 0 {
		t.Fatalf("active touches=%d want 0", active)
	}
}

func TestReleaseAllCount_ReleasesEverythingHeld(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"BUG-X", "BUG-Y", "BUG-Z"} {
		if err := s.Claim(ctx, id, "agent-a", "", nil, false); err != nil {
			t.Fatalf("claim %s: %v", id, err)
		}
	}

	n, err := s.ReleaseAllCount(ctx, "agent-a", "released")
	if err != nil {
		t.Fatalf("ReleaseAllCount: %v", err)
	}
	if n != 3 {
		t.Fatalf("released count=%d want 3", n)
	}
	var live int
	_ = db.QueryRow(`SELECT COUNT(*) FROM claims WHERE agent_id='agent-a'`).Scan(&live)
	if live != 0 {
		t.Fatalf("agent-a still holds %d claims", live)
	}
	var hist int
	_ = db.QueryRow(`SELECT COUNT(*) FROM claim_history WHERE agent_id='agent-a' AND outcome='released'`).Scan(&hist)
	if hist != 3 {
		t.Fatalf("history rows=%d want 3", hist)
	}
}

func TestReleaseAllCount_TolerantToVanishedRow(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"BUG-A", "BUG-B", "BUG-C"} {
		if err := s.Claim(ctx, id, "agent-a", "", nil, false); err != nil {
			t.Fatalf("claim %s: %v", id, err)
		}
	}
	// peer (force_release / reaper / direct) yanks one of agent-a's claims
	// between the snapshot read and the per-item delete.
	if _, err := db.Exec(`DELETE FROM claims WHERE item_id='BUG-B'`); err != nil {
		t.Fatal(err)
	}

	n, err := s.ReleaseAllCount(ctx, "agent-a", "released")
	if err != nil {
		t.Fatalf("ReleaseAllCount: %v", err)
	}
	if n != 2 {
		t.Fatalf("released count=%d want 2", n)
	}
	var live int
	_ = db.QueryRow(`SELECT COUNT(*) FROM claims WHERE agent_id='agent-a'`).Scan(&live)
	if live != 0 {
		t.Fatalf("agent-a still holds %d claims", live)
	}
}

func TestReleaseAllCount_NoErrorWhenPeerStealsClaimMidFlight(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	const trials = 30
	for trial := 0; trial < trials; trial++ {
		ids := []string{"R-A", "R-B", "R-C", "R-D"}
		for _, id := range ids {
			if err := s.Claim(ctx, id, "agent-a", "", nil, false); err != nil {
				t.Fatalf("trial %d claim %s: %v", trial, id, err)
			}
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.ForceRelease(ctx, "R-B", "agent-admin", "racing")
			_, _ = s.ForceRelease(ctx, "R-D", "agent-admin", "racing")
		}()
		_, err := s.ReleaseAllCount(ctx, "agent-a", "released")
		wg.Wait()
		if err != nil {
			t.Fatalf("trial %d: ReleaseAllCount returned error %v", trial, err)
		}
		// clean up any survivors via direct DB so the next trial starts clean
		_, _ = s.db.ExecContext(ctx, `DELETE FROM claims`)
	}
}

func TestRelease_AllowsReClaimAfterRelease(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	_ = s.Claim(ctx, "BUG-013", "agent-a", "", nil, false)
	_ = s.Release(ctx, "BUG-013", "agent-a", "released")

	if err := s.Claim(ctx, "BUG-013", "agent-b", "taking it", nil, false); err != nil {
		t.Fatalf("re-claim after release should succeed: %v", err)
	}
}
