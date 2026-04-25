package chat

import (
	"context"
	"strings"
	"testing"
)

func TestMailbox_Empty(t *testing.T) {
	c, _ := newTestChat(t)
	m, err := c.Mailbox(context.Background(), "agent-a")
	if err != nil {
		t.Fatalf("mailbox: %v", err)
	}
	if !m.Empty() {
		t.Fatalf("expected empty mailbox, got %+v", m)
	}
	if m.Format() != "" {
		t.Fatalf("empty mailbox must format to empty string, got %q", m.Format())
	}
}

func TestMailbox_FormatsByCategory(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := registerTestAgent(ctx, db, "repo-test", "peer", "Peer", 1700000000); err != nil {
		t.Fatal(err)
	}

	if err := c.Post(ctx, PostRequest{AgentID: "peer", Thread: ThreadGlobal, Kind: KindKnock, Priority: PriorityHigh, Body: "ping"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Post(ctx, PostRequest{AgentID: "peer", Thread: ThreadGlobal, Kind: KindSay, Body: "@agent-a status?"}); err != nil {
		t.Fatal(err)
	}

	m, err := c.Mailbox(ctx, "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	if m.Empty() {
		t.Fatal("expected non-empty mailbox")
	}
	out := m.Format()
	for _, want := range []string{"KNOCKS", "MENTIONS", "ping", "status?"} {
		if !strings.Contains(out, want) {
			t.Errorf("mailbox.Format() missing %q\n%s", want, out)
		}
	}
}

func TestMailbox_FormatJSON_NotifyShape(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	_ = registerTestAgent(ctx, db, "repo-test", "peer", "Peer", 1700000000)
	_ = c.Post(ctx, PostRequest{AgentID: "peer", Thread: ThreadGlobal, Kind: KindSay, Body: "@agent-a yo"})

	m, err := c.Mailbox(ctx, "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	js := m.FormatJSON()
	if !strings.Contains(js, `"decision":"block"`) {
		t.Errorf("expected decision:block, got %s", js)
	}
	if !strings.Contains(js, `"reason"`) {
		t.Errorf("expected reason field, got %s", js)
	}
}

func TestMailbox_MarkReadAdvancesCursors(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	_ = registerTestAgent(ctx, db, "repo-test", "peer", "Peer", 1700000000)
	_ = c.Post(ctx, PostRequest{AgentID: "peer", Thread: ThreadGlobal, Kind: KindSay, Body: "@agent-a one"})
	_ = c.Post(ctx, PostRequest{AgentID: "peer", Thread: ThreadGlobal, Kind: KindSay, Body: "@agent-a two"})

	m, err := c.Mailbox(ctx, "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	if m.Empty() {
		t.Fatal("expected unread")
	}
	if err := c.MarkMailboxRead(ctx, "agent-a", m); err != nil {
		t.Fatal(err)
	}

	again, err := c.Mailbox(ctx, "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	if !again.Empty() {
		t.Fatalf("expected empty after MarkMailboxRead, got %+v", again)
	}
}
