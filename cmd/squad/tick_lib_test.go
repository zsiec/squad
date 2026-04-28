package main

import (
	"context"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestTick_PureReturnsDigest(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if err := registerTestAgentInFixture(t, f, "agent-b", "B"); err != nil {
		t.Fatal(err)
	}
	if err := f.chat.Ask(ctx, "agent-b", chat.ThreadGlobal, f.agentID, "open up"); err != nil {
		t.Fatal(err)
	}
	res, err := Tick(ctx, TickArgs{Chat: f.chat, AgentID: f.agentID})
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if len(res.Digest.Mentions) == 0 {
		t.Fatalf("expected mention in digest: %+v", res.Digest)
	}
	body := strings.ToLower(res.Digest.Mentions[0].Body)
	if !strings.Contains(body, "open up") {
		t.Fatalf("mention body=%q", body)
	}
}

func TestTick_PureEmptyDigest(t *testing.T) {
	f := newChatFixture(t)
	res, err := Tick(context.Background(), TickArgs{Chat: f.chat, AgentID: f.agentID})
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if res == nil || res.Digest.Agent != f.agentID {
		t.Fatalf("digest=%+v", res)
	}
}
