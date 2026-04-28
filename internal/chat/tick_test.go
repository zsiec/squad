package chat

import (
	"context"
	"testing"
	"time"
)

func TestTick_AutoMarksReadAcrossThreads(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	_ = registerTestAgent(ctx, db, "repo-test", "agent-b", "B", c.nowUnix())
	_ = insertTestClaim(ctx, db, "repo-test", "BUG-1", "agent-b", "intent", c.nowUnix())

	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "g"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-1", Kind: KindFYI, Body: "i"})

	dg1, err := c.Tick(ctx, "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(dg1.Global) != 1 || len(dg1.YourThreads) != 1 {
		t.Fatalf("first tick missed messages: %+v", dg1)
	}

	dg2, err := c.Tick(ctx, "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(dg2.Global) != 0 || len(dg2.YourThreads) != 0 {
		t.Fatalf("second tick should be empty: %+v", dg2)
	}
}

func TestTick_BumpsHeartbeat(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	var before int64
	_ = db.QueryRow(`SELECT last_tick_at FROM agents WHERE id='agent-a'`).Scan(&before)

	c.now = func() time.Time { return time.Unix(before+60, 0) }

	if _, err := c.Tick(ctx, "agent-a"); err != nil {
		t.Fatal(err)
	}

	var after int64
	_ = db.QueryRow(`SELECT last_tick_at FROM agents WHERE id='agent-a'`).Scan(&after)
	if after <= before {
		t.Fatalf("heartbeat not bumped: before=%d after=%d", before, after)
	}
}

func TestTick_MentionNotRepeatedAfterRead(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	_ = registerTestAgent(ctx, db, "repo-test", "agent-b", "B", c.nowUnix())

	_ = c.Ask(ctx, "agent-a", ThreadGlobal, "agent-b", "ping")
	_, _ = c.Tick(ctx, "agent-b")

	dg, err := c.Tick(ctx, "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(dg.Mentions) != 0 {
		t.Fatalf("mention should not reappear after read: %+v", dg.Mentions)
	}
}
