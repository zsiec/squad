package chat

import (
	"context"
	"testing"
)

func TestResolveThread_ExplicitWins(t *testing.T) {
	c, _ := newTestChat(t)
	got, err := c.ResolveThread(context.Background(), "agent-a", "BUG-9")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "BUG-9" {
		t.Fatalf("got=%q want=BUG-9", got)
	}
}

func TestResolveThread_NoClaimFallsBackToGlobal(t *testing.T) {
	c, _ := newTestChat(t)
	got, err := c.ResolveThread(context.Background(), "agent-a", "")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
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
	got, err := c.ResolveThread(ctx, "agent-a", "")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
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
	got, err := c.ResolveThread(ctx, "agent-a", "")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != "BUG-2" {
		t.Fatalf("got=%q want=BUG-2", got)
	}
}

// A real DB failure (closed connection) must surface as a non-nil
// error and an empty thread — never silently fall through to
// ThreadGlobal, which would route messages to the wrong place.
func TestResolveThread_DBFailureDoesNotSilentlyFallBackToGlobal(t *testing.T) {
	c, db := newTestChat(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	got, err := c.ResolveThread(context.Background(), "agent-a", "")
	if err == nil {
		t.Fatalf("expected error from closed DB, got thread=%q err=nil", got)
	}
	if got != "" {
		t.Fatalf("error path must return empty thread; got=%q err=%v", got, err)
	}
}

// Override path must short-circuit before we touch the DB at all —
// proves the override priority survives a broken DB.
func TestResolveThread_ExplicitOverrideSkipsDB(t *testing.T) {
	c, db := newTestChat(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	got, err := c.ResolveThread(context.Background(), "agent-a", "BUG-9")
	if err != nil {
		t.Fatalf("override path must not query DB; err=%v", err)
	}
	if got != "BUG-9" {
		t.Fatalf("got=%q want=BUG-9", got)
	}
}
