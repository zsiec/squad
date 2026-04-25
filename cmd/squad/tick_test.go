package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestRunTick_ZeroExitOnEmpty(t *testing.T) {
	f := newChatFixture(t)
	if code := runTickBody(context.Background(), f.chat, f.agentID, false, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
}

func TestRunTick_PrintsKnocks(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if err := registerTestAgentInFixture(t, f, "agent-b", "B"); err != nil {
		t.Fatal(err)
	}
	if err := f.chat.Knock(ctx, "agent-b", f.agentID, "open up"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if code := runTickBody(ctx, f.chat, f.agentID, false, &buf); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := buf.String()
	if !strings.Contains(strings.ToUpper(out), "KNOCK") {
		t.Fatalf("expected knock in output, got %q", out)
	}
}

func TestRunTick_JSONFormat(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if err := registerTestAgentInFixture(t, f, "agent-b", "B"); err != nil {
		t.Fatal(err)
	}
	_ = f.chat.Post(ctx, chat.PostRequest{
		AgentID: "agent-b", Thread: chat.ThreadGlobal, Kind: chat.KindSay, Body: "hey",
	})
	var buf bytes.Buffer
	if code := runTickBody(ctx, f.chat, f.agentID, true, &buf); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Fatalf("expected JSON output, got %q", buf.String())
	}
}

func registerTestAgentInFixture(t *testing.T, f *chatFixture, id, name string) error {
	t.Helper()
	_, err := f.db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, '/tmp/wt', 1, 0, 0, 'active')
	`, id, f.repoID, name)
	return err
}
