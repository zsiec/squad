package chat

import (
	"context"
	"testing"
)

func TestResolveThread_ExplicitWins(t *testing.T) {
	c, _ := newTestChat(t)
	got := c.ResolveThread(context.Background(), "agent-a", "BUG-9")
	if got != "BUG-9" {
		t.Fatalf("got=%q want=BUG-9", got)
	}
}

func TestResolveThread_NoClaimFallsBackToGlobal(t *testing.T) {
	c, _ := newTestChat(t)
	got := c.ResolveThread(context.Background(), "agent-a", "")
	if got != ThreadGlobal {
		t.Fatalf("got=%q want=global", got)
	}
}

func TestResolveThread_UsesCurrentClaim(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	if err := insertTestClaim(ctx, db, "repo-test", "BUG-77", "agent-a", "intent", c.nowUnix()); err != nil {
		t.Fatal(err)
	}
	got := c.ResolveThread(ctx, "agent-a", "")
	if got != "BUG-77" {
		t.Fatalf("got=%q want=BUG-77", got)
	}
}

func TestResolveThread_PrefersMostRecentClaim(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	if err := insertTestClaim(ctx, db, "repo-test", "BUG-1", "agent-a", "first", 100); err != nil {
		t.Fatal(err)
	}
	if err := insertTestClaim(ctx, db, "repo-test", "BUG-2", "agent-a", "second", 200); err != nil {
		t.Fatal(err)
	}
	got := c.ResolveThread(ctx, "agent-a", "")
	if got != "BUG-2" {
		t.Fatalf("got=%q want=BUG-2", got)
	}
}
