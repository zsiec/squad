package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunKnock_RequiresTargetAndBody(t *testing.T) {
	f := newChatFixture(t)
	if code := runKnockBody(context.Background(), f.chat, f.agentID, []string{"@only-target"}); code == 0 {
		t.Fatal("expected non-zero for missing body")
	}
}

func TestRunKnock_PostsHighPriorityKnock(t *testing.T) {
	f := newChatFixture(t)
	if code := runKnockBody(context.Background(), f.chat, f.agentID, []string{"@agent-b", "are", "you", "there?"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	_, kind, _, mentions, priority := f.firstMessage(t)
	if kind != "knock" {
		t.Fatalf("kind=%q", kind)
	}
	if !strings.Contains(mentions, "agent-b") {
		t.Fatalf("mentions=%q", mentions)
	}
	if priority != "high" {
		t.Fatalf("priority=%q", priority)
	}
}
