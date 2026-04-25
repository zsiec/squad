package chat

import (
	"context"
	"testing"
)

func TestHistory_ReturnsThreadEntriesInTSOrder(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-1", Kind: KindSay, Body: "first"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-1", Kind: KindFYI, Body: "second"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-OTHER", Kind: KindSay, Body: "elsewhere"})

	entries, err := c.History(ctx, "BUG-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Body != "first" || entries[1].Body != "second" {
		t.Fatalf("order broken: %v", entries)
	}
}

func TestHistory_EmptyForUnknownItem(t *testing.T) {
	c, _ := newTestChat(t)
	entries, err := c.History(context.Background(), "BUG-NONE")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty, got %d", len(entries))
	}
}
