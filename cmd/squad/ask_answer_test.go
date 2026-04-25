package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunAsk_RequiresTargetAndBody(t *testing.T) {
	f := newChatFixture(t)
	if code := runAskBody(context.Background(), f.chat, f.agentID, "global", []string{"@agent-b"}, &bytes.Buffer{}); code == 0 {
		t.Fatal("expected non-zero on missing body")
	}
	if code := runAskBody(context.Background(), f.chat, f.agentID, "global", []string{"only-body", "x"}, &bytes.Buffer{}); code == 0 {
		t.Fatal("expected non-zero when first arg lacks @")
	}
}

func TestRunAsk_StoresAskKindAndMention(t *testing.T) {
	f := newChatFixture(t)
	var stdout bytes.Buffer
	if code := runAskBody(context.Background(), f.chat, f.agentID, "global", []string{"@agent-b", "why?"}, &stdout); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(stdout.String(), "[ask -> #global] @agent-b why?") {
		t.Fatalf("expected confirmation output, got %q", stdout.String())
	}
	_, kind, _, mentions, _ := f.firstMessage(t)
	if kind != "ask" {
		t.Fatalf("kind=%q", kind)
	}
	if !strings.Contains(mentions, "agent-b") {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestRunAnswer_PrependsReRef(t *testing.T) {
	f := newChatFixture(t)
	if code := runSayBody(context.Background(), f.chat, f.agentID, sayArgs{Body: "first ask"}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("setup say exit=%d", code)
	}
	if code := runAnswerBody(context.Background(), f.chat, f.agentID, []string{"1", "my", "reply"}); code != 0 {
		t.Fatalf("answer exit=%d", code)
	}
	var body string
	_ = f.db.QueryRow(`SELECT body FROM messages WHERE kind='answer'`).Scan(&body)
	if !strings.HasPrefix(body, "re:1 ") {
		t.Fatalf("body=%q", body)
	}
}
