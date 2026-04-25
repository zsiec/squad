package main

import (
	"context"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestRunSay_RejectsEmptyBody(t *testing.T) {
	f := newChatFixture(t)
	code := runSayBody(context.Background(), f.chat, f.agentID, sayArgs{Body: ""})
	if code == 0 {
		t.Fatal("expected non-zero exit on empty body")
	}
}

func TestRunSay_DefaultsToGlobal(t *testing.T) {
	f := newChatFixture(t)
	code := runSayBody(context.Background(), f.chat, f.agentID, sayArgs{Body: "hello team"})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	thread, kind, _, _, _ := f.firstMessage(t)
	if thread != "global" {
		t.Fatalf("thread=%q", thread)
	}
	if kind != chat.KindSay {
		t.Fatalf("kind=%q", kind)
	}
}

func TestRunSay_ExplicitMentions(t *testing.T) {
	f := newChatFixture(t)
	code := runSayBody(context.Background(), f.chat, f.agentID, sayArgs{
		Body:    "hi",
		Mention: "@alice,@bob",
	})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	_, _, _, mentions, _ := f.firstMessage(t)
	if !strings.Contains(mentions, "alice") || !strings.Contains(mentions, "bob") {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestRunSay_RoutesToCurrentClaim(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-123")
	code := runSayBody(context.Background(), f.chat, f.agentID, sayArgs{Body: "in-flight thought"})
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	thread, _, _, _, _ := f.firstMessage(t)
	if thread != "BUG-123" {
		t.Fatalf("thread=%q want BUG-123", thread)
	}
}
