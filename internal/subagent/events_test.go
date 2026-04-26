package subagent

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/zsiec/squad/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func insertAgent(t *testing.T, db *sql.DB, id, repoID string, lastTick int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO agents (id, repo_id, display_name, started_at, last_tick_at, status)
                       VALUES (?, ?, ?, ?, ?, 'active')`,
		id, repoID, id, lastTick, lastTick)
	require.NoError(t, err)
}

func TestRecord_BumpsHeartbeat(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	insertAgent(t, db, "agent-A", "repo-1", time.Now().Add(-time.Hour).Unix())
	fixedNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rec := New(db, "repo-1", func() time.Time { return fixedNow })
	require.NoError(t, rec.Record(ctx, Event{
		AgentID:    "agent-A",
		SubagentID: "sub-1",
		Type:       "Explore",
		EventName:  "subagent_start",
	}))
	var lastTick int64
	require.NoError(t, db.QueryRow(`SELECT last_tick_at FROM agents WHERE id=?`, "agent-A").Scan(&lastTick))
	require.Equal(t, fixedNow.Unix(), lastTick)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM subagent_events WHERE agent_id=?`, "agent-A").Scan(&count))
	require.Equal(t, 1, count)
}

func TestRecord_DurationFromStopPair(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	insertAgent(t, db, "agent-A", "repo-1", 100)
	startNow := time.Unix(10, 0)
	rec1 := New(db, "repo-1", func() time.Time { return startNow })
	require.NoError(t, rec1.Record(ctx, Event{
		AgentID: "agent-A", SubagentID: "sub-1", Type: "Explore", EventName: "subagent_start",
	}))
	stopNow := time.Unix(20, 0)
	rec2 := New(db, "repo-1", func() time.Time { return stopNow })
	require.NoError(t, rec2.Record(ctx, Event{
		AgentID: "agent-A", SubagentID: "sub-1", Type: "Explore", EventName: "subagent_stop",
	}))
	var dur sql.NullInt64
	require.NoError(t, db.QueryRow(`SELECT duration_ms FROM subagent_events WHERE event='subagent_stop' AND subagent_id='sub-1'`).Scan(&dur))
	require.True(t, dur.Valid)
	require.Equal(t, int64(10000), dur.Int64)
}

func TestRecord_NoAgentIsNoop(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	rec := New(db, "repo-1", time.Now)
	require.NoError(t, rec.Record(ctx, Event{
		AgentID: "agent-missing", EventName: "subagent_start",
	}))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM subagent_events`).Scan(&count))
	require.Equal(t, 0, count)
}
