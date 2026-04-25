package claims

import (
	"context"
	"errors"
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
