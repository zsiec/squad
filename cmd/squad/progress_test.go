package main

import (
	"bytes"
	"context"
	"testing"
)

func TestRunProgress_RequiresValidPct(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-9")
	if code := runProgressBody(context.Background(), f.db, f.repoID, f.chat, f.agentID, "BUG-9", "not-a-number", "", &bytes.Buffer{}); code == 0 {
		t.Fatal("expected non-zero on bad pct")
	}
}

func TestRunProgress_StoresWithNote(t *testing.T) {
	f := newChatFixture(t)
	f.insertClaim(t, "BUG-9")
	if code := runProgressBody(context.Background(), f.db, f.repoID, f.chat, f.agentID, "BUG-9", "60", "checkpoint", &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var pct int
	var note string
	_ = f.db.QueryRow(`SELECT pct, note FROM progress WHERE item_id='BUG-9'`).Scan(&pct, &note)
	if pct != 60 || note != "checkpoint" {
		t.Fatalf("pct=%d note=%q", pct, note)
	}
}

func TestRunProgress_RefusesNonHolder(t *testing.T) {
	f := newChatFixture(t)
	// Insert claim held by a DIFFERENT agent
	if _, err := f.db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, 'agent-other', 0, 0, '', 0)
	`, f.repoID, "BUG-9"); err != nil {
		t.Fatal(err)
	}
	if code := runProgressBody(context.Background(), f.db, f.repoID, f.chat, f.agentID, "BUG-9", "50", "", &bytes.Buffer{}); code == 0 {
		t.Fatal("expected non-zero when not the holder")
	}
}

func TestRunProgress_RefusesUnclaimed(t *testing.T) {
	f := newChatFixture(t)
	if code := runProgressBody(context.Background(), f.db, f.repoID, f.chat, f.agentID, "BUG-NEVER", "50", "", &bytes.Buffer{}); code == 0 {
		t.Fatal("expected non-zero on unclaimed item")
	}
}
