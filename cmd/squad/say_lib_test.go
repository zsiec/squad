package main

import (
	"context"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestSay_PurePostsToResolvedThread(t *testing.T) {
	f := newChatFixture(t)
	res, err := Say(context.Background(), SayArgs{
		Chat:    f.chat,
		AgentID: f.agentID,
		To:      chat.ThreadGlobal,
		Body:    "hello team",
	})
	if err != nil {
		t.Fatalf("Say: %v", err)
	}
	if res == nil || res.To != chat.ThreadGlobal || res.Body != "hello team" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.PostedAt == 0 {
		t.Fatalf("PostedAt unset")
	}

	thread, kind, body, _, _ := f.firstMessage(t)
	if thread != chat.ThreadGlobal {
		t.Fatalf("thread=%q", thread)
	}
	if kind != chat.KindSay {
		t.Fatalf("kind=%q", kind)
	}
	if body != "hello team" {
		t.Fatalf("body=%q", body)
	}
}

func TestSay_PureExplicitMentions(t *testing.T) {
	f := newChatFixture(t)
	res, err := Say(context.Background(), SayArgs{
		Chat:     f.chat,
		AgentID:  f.agentID,
		To:       chat.ThreadGlobal,
		Body:     "hi",
		Mentions: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("Say: %v", err)
	}
	if len(res.Mentions) != 2 {
		t.Fatalf("res mentions=%v", res.Mentions)
	}
	_, _, _, mentions, _ := f.firstMessage(t)
	if !strings.Contains(mentions, "alice") || !strings.Contains(mentions, "bob") {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestSay_PureRejectsEmptyBody(t *testing.T) {
	f := newChatFixture(t)
	_, err := Say(context.Background(), SayArgs{
		Chat:    f.chat,
		AgentID: f.agentID,
		To:      chat.ThreadGlobal,
		Body:    "   ",
	})
	if err == nil {
		t.Fatal("expected error on empty body")
	}
}
