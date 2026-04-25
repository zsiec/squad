package claims

import (
	"context"
	"errors"
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
	db.QueryRow(`SELECT COUNT(*) FROM claims WHERE item_id='BUG-010'`).Scan(&live)
	if live != 0 {
		t.Fatalf("claim still active, count=%d", live)
	}
	var outcome string
	db.QueryRow(`SELECT outcome FROM claim_history WHERE item_id='BUG-010'`).Scan(&outcome)
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
	db.QueryRow(`SELECT COUNT(*) FROM touches WHERE item_id='BUG-012' AND released_at IS NULL`).Scan(&active)
	if active != 0 {
		t.Fatalf("active touches=%d want 0", active)
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
