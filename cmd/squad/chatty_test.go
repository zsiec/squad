package main

import (
	"context"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestChatty_DefaultsToCurrentClaim(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-77")
	if code := runChatty(context.Background(), f.chat, f.agentID, chat.KindThinking, "", "", []string{"head's somewhere"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	thread, kind, _, _, _ := f.firstMessage(t)
	if thread != "BUG-77" {
		t.Fatalf("thread=%q", thread)
	}
	if kind != chat.KindThinking {
		t.Fatalf("kind=%q", kind)
	}
}

func TestChatty_FallsBackToGlobalWithNoClaim(t *testing.T) {
	f := newChatFixture(t)
	if code := runChatty(context.Background(), f.chat, f.agentID, chat.KindStuck, "", "", []string{"need help"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	thread, _, _, _, _ := f.firstMessage(t)
	if thread != "global" {
		t.Fatalf("thread=%q", thread)
	}
}

func TestChatty_ExplicitToOverridesClaim(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-77")
	if code := runChatty(context.Background(), f.chat, f.agentID, chat.KindFYI, "global", "", []string{"heads up"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	thread, _, _, _, _ := f.firstMessage(t)
	if thread != "global" {
		t.Fatalf("thread=%q", thread)
	}
}

func TestChatty_RejectsEmptyBody(t *testing.T) {
	f := newChatFixture(t)
	if code := runChatty(context.Background(), f.chat, f.agentID, chat.KindThinking, "", "", []string{}); code == 0 {
		t.Fatal("expected non-zero")
	}
}
