package main

import (
	"context"
	"testing"
)

func TestRunProgress_RequiresValidPct(t *testing.T) {
	f := newChatFixture(t)
	if code := runProgressBody(context.Background(), f.chat, f.agentID, "BUG-9", "not-a-number", ""); code == 0 {
		t.Fatal("expected non-zero on bad pct")
	}
}

func TestRunProgress_StoresWithNote(t *testing.T) {
	f := newChatFixture(t)
	if code := runProgressBody(context.Background(), f.chat, f.agentID, "BUG-9", "60", "checkpoint"); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var pct int
	var note string
	_ = f.db.QueryRow(`SELECT pct, note FROM progress WHERE item_id='BUG-9'`).Scan(&pct, &note)
	if pct != 60 || note != "checkpoint" {
		t.Fatalf("pct=%d note=%q", pct, note)
	}
}
