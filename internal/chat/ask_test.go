package chat

import (
	"context"
	"strings"
	"testing"
)

func TestAsk_StoresAskKindAndForcesMention(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.Ask(ctx, "agent-a", ThreadGlobal, "agent-b", "why pick 30 threshold?"); err != nil {
		t.Fatal(err)
	}

	var kind, mentions string
	_ = db.QueryRow(`SELECT kind, mentions FROM messages`).Scan(&kind, &mentions)
	if kind != KindAsk {
		t.Fatalf("kind=%q", kind)
	}
	if !strings.Contains(mentions, "agent-b") {
		t.Fatalf("mentions=%q", mentions)
	}
}
