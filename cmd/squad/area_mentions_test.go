package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// seedDoneInArea inserts an item + a 'done' claim_history row attributing
// the close to agentID in the given area. Mirrors the schema columns used
// by stats.TopCloser.
func seedDoneInArea(t *testing.T, db *sql.DB, repoID, itemID, area, agentID string, releasedAt int64) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO items (repo_id, item_id, title, type, priority, area,
			status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES (?, ?, 't', 'feat', 'P2', ?, 'done', '', '', 0, 0, 0, '', ?)`,
		repoID, itemID, area, releasedAt,
	); err != nil {
		t.Fatalf("seed item %s: %v", itemID, err)
	}
	if _, err := db.Exec(`
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, ?, ?, ?, ?, 'done')`,
		repoID, itemID, agentID, releasedAt-100, releasedAt,
	); err != nil {
		t.Fatalf("seed claim_history %s: %v", itemID, err)
	}
}

func TestNotifyAreaTopCloser_PostsFyiWhenAgentQualifies(t *testing.T) {
	f := newChatFixture(t)
	for i, id := range []string{"FEAT-1", "FEAT-2", "FEAT-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "chat", "agent-other", time.Now().Add(time.Duration(i)*time.Second).Unix())
	}

	notifyAreaTopCloser(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", chat.ThreadGlobal, "new FEAT-9 in area chat — heads-up")

	thread, kind, body, mentions, _ := f.firstMessage(t)
	if thread != chat.ThreadGlobal {
		t.Errorf("thread=%q want global", thread)
	}
	if kind != chat.KindFYI {
		t.Errorf("kind=%q want fyi", kind)
	}
	if body == "" || body[:3] != "new" {
		t.Errorf("body=%q", body)
	}
	if !strings.Contains(mentions, "agent-other") {
		t.Errorf("mentions=%q want to contain agent-other", mentions)
	}
}

func TestNotifyAreaTopCloser_SilentWhenNoQualifier(t *testing.T) {
	f := newChatFixture(t)
	// Only 2 closes: below threshold of 3.
	seedDoneInArea(t, f.db, f.repoID, "FEAT-1", "chat", "agent-other", time.Now().Unix())
	seedDoneInArea(t, f.db, f.repoID, "FEAT-2", "chat", "agent-other", time.Now().Unix()+1)

	notifyAreaTopCloser(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", chat.ThreadGlobal, "new FEAT-9")

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages, got %d", n)
	}
}

func TestNotifyAreaTopCloser_SkipsWhenSelfIsTopCloser(t *testing.T) {
	f := newChatFixture(t)
	// Current agent is the top closer — don't @mention yourself.
	for i, id := range []string{"FEAT-1", "FEAT-2", "FEAT-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "chat", f.agentID, time.Now().Add(time.Duration(i)*time.Second).Unix())
	}

	notifyAreaTopCloser(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", chat.ThreadGlobal, "new FEAT-9")

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages (self-mention suppressed), got %d", n)
	}
}

func TestNotifyAreaTopCloser_SuppressedByEnv(t *testing.T) {
	f := newChatFixture(t)
	for i, id := range []string{"FEAT-1", "FEAT-2", "FEAT-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "chat", "agent-other", time.Now().Add(time.Duration(i)*time.Second).Unix())
	}

	t.Setenv("SQUAD_NO_AREA_MENTIONS", "1")
	notifyAreaTopCloser(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", chat.ThreadGlobal, "new FEAT-9")

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages with SQUAD_NO_AREA_MENTIONS=1, got %d", n)
	}
}

func TestNotifyAreaChange_PostsFyiOnNewAreaTopCloser(t *testing.T) {
	f := newChatFixture(t)
	now := time.Now().Unix()
	// agent-other dominates "stats". area "chat" has no qualifying closer.
	for i, id := range []string{"OLD-1", "OLD-2", "OLD-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "stats", "agent-other", now-int64(i*60))
	}

	notifyAreaChange(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", "stats", "FEAT-9", "FEAT-9 area changed to stats")

	thread, kind, _, mentions, _ := f.firstMessage(t)
	if thread != "FEAT-9" {
		t.Errorf("thread=%q want FEAT-9", thread)
	}
	if kind != chat.KindFYI {
		t.Errorf("kind=%q want fyi", kind)
	}
	if !strings.Contains(mentions, "agent-other") {
		t.Errorf("mentions=%q want agent-other", mentions)
	}
}

func TestNotifyAreaChange_NoOpWhenAreaUnchanged(t *testing.T) {
	f := newChatFixture(t)
	now := time.Now().Unix()
	for i, id := range []string{"OLD-1", "OLD-2", "OLD-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "chat", "agent-other", now-int64(i*60))
	}
	notifyAreaChange(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", "chat", "FEAT-9", "noop")

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages on no-area-change, got %d", n)
	}
}

func TestNotifyAreaChange_SuppressesIfSameTopCloser(t *testing.T) {
	f := newChatFixture(t)
	now := time.Now().Unix()
	// agent-other tops both areas — no new routing signal, so suppress.
	for i, id := range []string{"OLD-1", "OLD-2", "OLD-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "chat", "agent-other", now-int64(i*60))
	}
	for i, id := range []string{"OLD-4", "OLD-5", "OLD-6"} {
		seedDoneInArea(t, f.db, f.repoID, id, "stats", "agent-other", now-int64(i*60+200))
	}

	notifyAreaChange(context.Background(), f.db, f.chat, f.repoID, f.agentID, "chat", "stats", "FEAT-9", "x")

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages (same top closer in old & new area), got %d", n)
	}
}

func TestNotifyAreaTopCloser_PlaceholderAreaSkipped(t *testing.T) {
	f := newChatFixture(t)
	// Even with qualifying closes in area "<fill-in>", placeholder areas
	// must not trigger a mention — they're the new-item scaffold default.
	for i, id := range []string{"FEAT-1", "FEAT-2", "FEAT-3"} {
		seedDoneInArea(t, f.db, f.repoID, id, "<fill-in>", "agent-other", time.Now().Add(time.Duration(i)*time.Second).Unix())
	}

	notifyAreaTopCloser(context.Background(), f.db, f.chat, f.repoID, f.agentID, "<fill-in>", chat.ThreadGlobal, "new FEAT-9")

	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 messages for placeholder area, got %d", n)
	}
}
