package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

func TestRunHandoff_RejectsEmpty(t *testing.T) {
	f := newChatFixture(t)
	st := claims.New(f.db, f.repoID, func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) })
	if code := runHandoffBody(context.Background(), f.chat, st, f.db, f.repoID, "", f.agentID, chat.HandoffBody{}); code == 0 {
		t.Fatal("expected non-zero")
	}
}

func TestRunHandoff_StoresAndReleasesClaim(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-9")
	st := claims.New(f.db, f.repoID, func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) })

	h := chat.HandoffBody{Shipped: []string{"BUG-9"}, Note: "headed to lunch"}
	if code := runHandoffBody(context.Background(), f.chat, st, f.db, f.repoID, "", f.agentID, h); code != 0 {
		t.Fatalf("exit=%d", code)
	}

	var body string
	_ = f.db.QueryRow(`SELECT body FROM messages WHERE kind='handoff'`).Scan(&body)
	if !strings.Contains(body, "BUG-9") {
		t.Fatalf("body=%q", body)
	}

	var open int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM claims WHERE agent_id = ?`, f.agentID).Scan(&open)
	if open != 0 {
		t.Fatalf("claim still open: count=%d", open)
	}
}
