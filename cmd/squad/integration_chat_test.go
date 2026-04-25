package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

// TestChatIntegration_TwoAgentsConverse runs every chat verb against a single
// shared store and checks that they cooperate end-to-end: a knock from agent-a
// surfaces in agent-b's tick, a progress note for an item shows up via
// LatestProgress, and a thinking/handoff round-trip is durably stored.
func TestChatIntegration_TwoAgentsConverse(t *testing.T) {
	f := newChatFixture(t)
	ctx := context.Background()

	const agentB = "agent-b"
	if err := registerTestAgentInFixture(t, f, agentB, "B"); err != nil {
		t.Fatal(err)
	}

	// agent-a (== f.agentID) knocks agent-b.
	if code := runKnockBody(ctx, f.chat, f.agentID, []string{"@" + agentB, "stand", "by"}); code != 0 {
		t.Fatalf("knock exit=%d", code)
	}

	// agent-b ticks and should see the knock in the digest output.
	var buf bytes.Buffer
	if code := runTickBody(ctx, f.chat, agentB, false, &buf); code != 0 {
		t.Fatalf("tick exit=%d", code)
	}
	if !strings.Contains(buf.String(), "stand by") {
		t.Fatalf("knock not in tick output: %q", buf.String())
	}

	// agent-b reports progress on BUG-1.
	if _, err := f.db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, 'BUG-1', ?, 0, 0, '', 0)
	`, f.repoID, agentB); err != nil {
		t.Fatal(err)
	}
	if code := runProgressBody(ctx, f.db, f.repoID, f.chat, agentB, "BUG-1", "40", "almost there", &bytes.Buffer{}); code != 0 {
		t.Fatalf("progress exit=%d", code)
	}
	pct, note := f.chat.LatestProgress(ctx, "BUG-1")
	if pct != 40 || note != "almost there" {
		t.Fatalf("progress: pct=%d note=%q", pct, note)
	}

	// agent-a posts a thinking message into BUG-1; verify via History.
	if code := runChatty(ctx, f.chat, f.agentID, chat.KindThinking, "BUG-1", "", []string{"leaning toward approach Z"}); code != 0 {
		t.Fatalf("thinking exit=%d", code)
	}
	hist, err := f.chat.History(ctx, "BUG-1")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range hist {
		if h.Kind == chat.KindThinking && strings.Contains(h.Body, "approach Z") {
			found = true
		}
	}
	if !found {
		t.Fatalf("thinking missing from history: %+v", hist)
	}

	// agent-a posts a handoff with a note + in-flight item; verify body persisted.
	st := claims.New(f.db, f.repoID, func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) })
	h := chat.HandoffBody{InFlight: []string{"BUG-1"}, Note: "lunchtime"}
	if code := runHandoffBody(ctx, f.chat, st, f.agentID, h); code != 0 {
		t.Fatalf("handoff exit=%d", code)
	}
	var body string
	_ = f.db.QueryRow(`SELECT body FROM messages WHERE kind='handoff'`).Scan(&body)
	if !strings.Contains(body, "BUG-1") || !strings.Contains(body, "lunchtime") {
		t.Fatalf("handoff body=%q", body)
	}
}
