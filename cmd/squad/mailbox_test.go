package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/chat"
)

func TestRunMailbox_EmptyExitsClean(t *testing.T) {
	f := newChatFixture(t)
	var stdout bytes.Buffer
	code := runMailbox(context.Background(), f.chat, f.agentID, mailboxArgs{Format: "additional-context"}, &stdout)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected silent stdout on empty mailbox, got %q", stdout.String())
	}
}

func TestRunMailbox_AdditionalContextEnvelope(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if _, err := f.db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('peer', ?, 'Peer', '/tmp/wt', 1, 1, 1, 'active')
	`, f.repoID); err != nil {
		t.Fatal(err)
	}
	if err := f.chat.Post(ctx, chat.PostRequest{
		AgentID: "peer", Thread: chat.ThreadGlobal, Kind: chat.KindSay,
		Body: "@" + f.agentID + " yo",
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := runMailbox(ctx, f.chat, f.agentID, mailboxArgs{
		Format: "additional-context",
		Event:  "PostToolUse",
	}, &stdout)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var env struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout.String())
	}
	if env.HookSpecificOutput.HookEventName != "PostToolUse" {
		t.Fatalf("event=%q", env.HookSpecificOutput.HookEventName)
	}
	if !strings.Contains(env.HookSpecificOutput.AdditionalContext, "yo") {
		t.Fatalf("context missing body: %q", env.HookSpecificOutput.AdditionalContext)
	}
}

func TestRunMailbox_DecisionBlockFormat(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()
	if _, err := f.db.Exec(`
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES ('peer', ?, 'Peer', '/tmp/wt', 1, 1, 1, 'active')
	`, f.repoID); err != nil {
		t.Fatal(err)
	}
	_ = f.chat.Post(ctx, chat.PostRequest{
		AgentID: "peer", Thread: chat.ThreadGlobal, Kind: chat.KindSay,
		Body: "@" + f.agentID + " hi",
	})

	var stdout bytes.Buffer
	code := runMailbox(ctx, f.chat, f.agentID, mailboxArgs{Format: "decision-block"}, &stdout)
	if code != 2 {
		t.Fatalf("decision-block must exit 2, got %d", code)
	}
	if !strings.Contains(stdout.String(), `"decision":"block"`) {
		t.Fatalf("missing decision:block in %s", stdout.String())
	}
}
