package main

import (
	"context"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestAsk_PurePostsAskOnGlobal(t *testing.T) {
	f := newChatFixture(t)
	res, err := Ask(context.Background(), AskArgs{
		Chat:     f.chat,
		AgentID:  f.agentID,
		To:       chat.ThreadGlobal,
		Target:   "agent-b",
		Question: "why?",
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if res == nil || res.Target != "agent-b" || res.To != chat.ThreadGlobal {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.PostedAt == 0 {
		t.Fatalf("PostedAt unset")
	}

	thread, kind, body, mentions, _ := f.firstMessage(t)
	if thread != chat.ThreadGlobal {
		t.Fatalf("thread=%q", thread)
	}
	if kind != chat.KindAsk {
		t.Fatalf("kind=%q", kind)
	}
	if !strings.Contains(body, "@agent-b why?") {
		t.Fatalf("body=%q", body)
	}
	if !strings.Contains(mentions, "agent-b") {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestAsk_PureRejectsEmptyTarget(t *testing.T) {
	f := newChatFixture(t)
	_, err := Ask(context.Background(), AskArgs{
		Chat:     f.chat,
		AgentID:  f.agentID,
		To:       chat.ThreadGlobal,
		Target:   "",
		Question: "huh",
	})
	if err == nil {
		t.Fatal("expected error on empty target")
	}
}
