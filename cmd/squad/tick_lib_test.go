package main

import (
	"context"
	"strings"
	"testing"
)

func TestTick_PureReturnsDigest(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if err := registerTestAgentInFixture(t, f, "agent-b", "B"); err != nil {
		t.Fatal(err)
	}
	if err := f.chat.Knock(ctx, "agent-b", f.agentID, "open up"); err != nil {
		t.Fatal(err)
	}
	res, err := Tick(ctx, TickArgs{Chat: f.chat, AgentID: f.agentID})
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if len(res.Digest.Knocks) == 0 {
		t.Fatalf("expected knock in digest: %+v", res.Digest)
	}
	body := strings.ToLower(res.Digest.Knocks[0].Body)
	if !strings.Contains(body, "open up") {
		t.Fatalf("knock body=%q", body)
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
