package chat

import (
	"context"
	"strings"
	"testing"
)

func TestKnock_HighPriorityWithTargetMention(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.Knock(ctx, "agent-a", "agent-b", "got a sec?"); err != nil {
		t.Fatal(err)
	}

	var kind, priority, mentions string
	_ = db.QueryRow(`SELECT kind, priority, mentions FROM messages`).Scan(&kind, &priority, &mentions)
	if kind != KindKnock {
		t.Fatalf("kind=%q", kind)
	}
	if priority != PriorityHigh {
		t.Fatalf("priority=%q", priority)
	}
	if !strings.Contains(mentions, "agent-b") {
		t.Fatalf("mentions=%q", mentions)
	}
}
