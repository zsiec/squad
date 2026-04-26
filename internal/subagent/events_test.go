package subagent

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/zsiec/squad/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(dir + "/test.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func insertAgent(t *testing.T, db *sql.DB, id, repoID string, lastTick int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO agents (id, repo_id, display_name, started_at, last_tick_at, status)
                       VALUES (?, ?, ?, ?, ?, 'active')`,
		id, repoID, id, lastTick, lastTick); err != nil {
		t.Fatalf("insert agent %s: %v", id, err)
	}
}

func TestRecord_BumpsHeartbeat(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	insertAgent(t, db, "agent-A", "repo-1", time.Now().Add(-time.Hour).Unix())
	fixedNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rec := New(db, "repo-1", func() time.Time { return fixedNow })
	if err := rec.Record(ctx, Event{
		AgentID:    "agent-A",
		SubagentID: "sub-1",
		Type:       "Explore",
		EventName:  "subagent_start",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	var lastTick int64
	if err := db.QueryRow(`SELECT last_tick_at FROM agents WHERE id=?`, "agent-A").Scan(&lastTick); err != nil {
		t.Fatalf("query last_tick_at: %v", err)
	}
	if lastTick != fixedNow.Unix() {
		t.Fatalf("last_tick_at=%d want %d", lastTick, fixedNow.Unix())
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM subagent_events WHERE agent_id=?`, "agent-A").Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 1 {
		t.Fatalf("event count=%d want 1", count)
	}
}

func TestRecord_DurationFromStopPair(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	insertAgent(t, db, "agent-A", "repo-1", 100)
	startNow := time.Unix(10, 0)
	rec1 := New(db, "repo-1", func() time.Time { return startNow })
	if err := rec1.Record(ctx, Event{
		AgentID: "agent-A", SubagentID: "sub-1", Type: "Explore", EventName: "subagent_start",
	}); err != nil {
		t.Fatalf("record start: %v", err)
	}
	stopNow := time.Unix(20, 0)
	rec2 := New(db, "repo-1", func() time.Time { return stopNow })
	if err := rec2.Record(ctx, Event{
		AgentID: "agent-A", SubagentID: "sub-1", Type: "Explore", EventName: "subagent_stop",
	}); err != nil {
		t.Fatalf("record stop: %v", err)
	}
	var dur sql.NullInt64
	if err := db.QueryRow(`SELECT duration_ms FROM subagent_events WHERE event='subagent_stop' AND subagent_id='sub-1'`).Scan(&dur); err != nil {
		t.Fatalf("query duration: %v", err)
	}
	if !dur.Valid {
		t.Fatalf("duration_ms not valid")
	}
	if dur.Int64 != 10000 {
		t.Fatalf("duration_ms=%d want 10000", dur.Int64)
	}
}

func TestRecord_NoAgentIsNoop(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	rec := New(db, "repo-1", time.Now)
	if err := rec.Record(ctx, Event{
		AgentID: "agent-missing", EventName: "subagent_start",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM subagent_events`).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 0 {
		t.Fatalf("event count=%d want 0", count)
	}
}
