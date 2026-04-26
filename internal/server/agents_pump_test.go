package server

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func seedAgent(t *testing.T, db *sql.DB, repoID, agentID string, lastTickAt int64, status string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO agents (id, repo_id, display_name, started_at, last_tick_at, status) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET last_tick_at=excluded.last_tick_at, status=excluded.status`,
		agentID, repoID, agentID, lastTickAt, lastTickAt, status,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSE_AgentStatus_Appeared(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	time.Sleep(700 * time.Millisecond)
	seedAgent(t, db, testRepoID, "agent-new", time.Now().Unix(), "active")

	if !waitSSEEvent(t, resp.Body, "agent_status", 3*time.Second) {
		t.Fatal("did not see agent_status event after new agent insert")
	}
}

func TestSSE_AgentStatus_TickUpdated(t *testing.T) {
	db := newTestDB(t)
	// pre-seed an agent before server start so it's part of the baseline snapshot
	if _, err := db.Exec(
		`INSERT INTO agents (id, repo_id, display_name, started_at, last_tick_at, status) VALUES (?, ?, ?, ?, ?, ?)`,
		"agent-pre", testRepoID, "agent-pre", time.Now().Unix()-60, time.Now().Unix()-60, "idle",
	); err != nil {
		t.Fatal(err)
	}

	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	time.Sleep(700 * time.Millisecond)

	// Tick the pre-seeded agent — last_tick_at and status change.
	seedAgent(t, db, testRepoID, "agent-pre", time.Now().Unix(), "active")

	if !waitSSEEvent(t, resp.Body, "agent_status", 3*time.Second) {
		t.Fatal("did not see agent_status event after tick update")
	}
}
