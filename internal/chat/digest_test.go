package chat

import (
	"context"
	"testing"
)

func TestDigest_BucketsKnocksAndMentions(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	if err := registerTestAgent(ctx, db, "repo-test", "agent-b", "B", c.nowUnix()); err != nil {
		t.Fatal(err)
	}

	_ = c.Knock(ctx, "agent-a", "agent-b", "ping")
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "@agent-b plain mention"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "no mention here"})

	dg, err := c.Digest(ctx, "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(dg.Knocks) != 1 {
		t.Fatalf("knocks=%d", len(dg.Knocks))
	}
	if len(dg.Mentions) != 1 {
		t.Fatalf("mentions=%d", len(dg.Mentions))
	}
	if len(dg.Global) != 1 {
		t.Fatalf("global=%d", len(dg.Global))
	}
}

func TestDigest_YourThreadsForActiveClaims(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	_ = registerTestAgent(ctx, db, "repo-test", "agent-b", "B", c.nowUnix())
	_ = insertTestClaim(ctx, db, "repo-test", "BUG-1", "agent-b", "intent", c.nowUnix())

	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-1", Kind: KindFYI, Body: "in your item"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-OTHER", Kind: KindFYI, Body: "elsewhere"})

	dg, err := c.Digest(ctx, "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(dg.YourThreads) != 1 {
		t.Fatalf("your_threads=%d", len(dg.YourThreads))
	}
	if dg.YourThreads[0].Thread != "BUG-1" {
		t.Fatalf("thread=%q", dg.YourThreads[0].Thread)
	}
}

func TestDigest_SelfMessagesExcluded(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()

	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "self"})

	dg, err := c.Digest(ctx, "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	total := len(dg.Knocks) + len(dg.Mentions) + len(dg.Global) + len(dg.Handoffs)
	if total != 0 {
		t.Fatalf("self message leaked, total=%d", total)
	}
}

func TestDigest_RespectsReadCursor(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	_ = registerTestAgent(ctx, db, "repo-test", "agent-b", "B", c.nowUnix())

	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "before"})
	_, _ = db.Exec(`INSERT INTO reads (agent_id, thread, last_msg_id) VALUES ('agent-b', 'global', (SELECT MAX(id) FROM messages))`)
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "after"})

	dg, err := c.Digest(ctx, "agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(dg.Global) != 1 || dg.Global[0].Body != "after" {
		t.Fatalf("digest=%+v", dg.Global)
	}
}
