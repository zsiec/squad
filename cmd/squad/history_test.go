package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestRunHistory_RequiresItem(t *testing.T) {
	f := newChatFixture(t)
	if code := runHistoryBody(context.Background(), f.chat, "", &bytes.Buffer{}); code == 0 {
		t.Fatal("expected non-zero")
	}
}

func TestRunHistory_PrintsEntries(t *testing.T) {
	f := newChatFixture(t)
	_ = f.chat.Post(context.Background(), chat.PostRequest{
		AgentID: f.agentID, Thread: "BUG-9", Kind: chat.KindSay, Body: "the entry",
	})
	var buf bytes.Buffer
	if code := runHistoryBody(context.Background(), f.chat, "BUG-9", &buf); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(buf.String(), "the entry") {
		t.Fatalf("missing body: %q", buf.String())
	}
}
