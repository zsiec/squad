package main

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
)

func countSubagentEvents(t *testing.T, db *sql.DB, agentID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM subagent_events WHERE agent_id = ?`, agentID,
	).Scan(&n); err != nil {
		t.Fatalf("count subagent_events: %v", err)
	}
	return n
}

func runSubagentEvent(t *testing.T, env *testEnv, stdin string) {
	t.Helper()
	t.Chdir(env.Root)
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs([]string{"subagent-event"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v: stderr=%s", err, errOut.String())
	}
}

func registerSquadAgent(t *testing.T, env *testEnv) {
	t.Helper()
	if _, err := env.DB.Exec(
		`INSERT INTO agents (id, repo_id, display_name, started_at, last_tick_at, status)
         VALUES (?, ?, ?, 100, 100, 'active')`,
		env.AgentID, env.RepoID, env.AgentID,
	); err != nil {
		t.Fatalf("insert agent: %v", err)
	}
}

func TestSubagentEvent_RecordsSubagentStart(t *testing.T) {
	env := newTestEnv(t)
	registerSquadAgent(t, env)
	payload := `{"hook_event_name":"SubagentStart","agent_id":"sub-1","agent_type":"Explore"}`
	runSubagentEvent(t, env, payload)

	if got := countSubagentEvents(t, env.DB, env.AgentID); got != 1 {
		t.Fatalf("event count: got %d want 1", got)
	}
	var event, subID, subType string
	if err := env.DB.QueryRow(
		`SELECT event, subagent_id, subagent_type FROM subagent_events WHERE agent_id=?`,
		env.AgentID,
	).Scan(&event, &subID, &subType); err != nil {
		t.Fatalf("select: %v", err)
	}
	if event != "subagent_start" {
		t.Fatalf("event=%q want subagent_start", event)
	}
	if subID != "sub-1" {
		t.Fatalf("subagent_id=%q want sub-1", subID)
	}
	if subType != "Explore" {
		t.Fatalf("subagent_type=%q want Explore", subType)
	}
}

func TestSubagentEvent_StopAfterStartHasDuration(t *testing.T) {
	env := newTestEnv(t)
	registerSquadAgent(t, env)
	runSubagentEvent(t, env, `{"hook_event_name":"SubagentStart","agent_id":"sub-2","agent_type":"general-purpose"}`)
	runSubagentEvent(t, env, `{"hook_event_name":"SubagentStop","agent_id":"sub-2","agent_type":"general-purpose"}`)

	var dur sql.NullInt64
	if err := env.DB.QueryRow(
		`SELECT duration_ms FROM subagent_events WHERE event='subagent_stop' AND subagent_id='sub-2'`,
	).Scan(&dur); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !dur.Valid {
		t.Fatalf("duration_ms NULL; want set")
	}
	if dur.Int64 < 0 {
		t.Fatalf("duration_ms=%d want >=0", dur.Int64)
	}
}

func TestSubagentEvent_EmptyStdinNoop(t *testing.T) {
	env := newTestEnv(t)
	registerSquadAgent(t, env)
	runSubagentEvent(t, env, "")
	if got := countSubagentEvents(t, env.DB, env.AgentID); got != 0 {
		t.Fatalf("event count: got %d want 0", got)
	}
}

func TestSubagentEvent_MalformedJSONNoop(t *testing.T) {
	env := newTestEnv(t)
	registerSquadAgent(t, env)
	runSubagentEvent(t, env, "not-json{")
	if got := countSubagentEvents(t, env.DB, env.AgentID); got != 0 {
		t.Fatalf("event count: got %d want 0", got)
	}
}

func TestSubagentEvent_UnknownHookEventNameNoop(t *testing.T) {
	env := newTestEnv(t)
	registerSquadAgent(t, env)
	runSubagentEvent(t, env, `{"hook_event_name":"WeirdNewEvent","agent_id":"sub-x"}`)
	if got := countSubagentEvents(t, env.DB, env.AgentID); got != 0 {
		t.Fatalf("event count: got %d want 0", got)
	}
}

func TestSubagentEvent_RecordsTaskCreatedAndCompleted(t *testing.T) {
	env := newTestEnv(t)
	registerSquadAgent(t, env)
	runSubagentEvent(t, env, `{"hook_event_name":"TaskCreated","agent_id":"task-7"}`)
	runSubagentEvent(t, env, `{"hook_event_name":"TaskCompleted","agent_id":"task-7"}`)

	rows, err := env.DB.Query(
		`SELECT event FROM subagent_events WHERE subagent_id='task-7' ORDER BY id`,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var ev string
		if err := rows.Scan(&ev); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, ev)
	}
	want := []string{"task_created", "task_completed"}
	if len(got) != len(want) {
		t.Fatalf("events=%v want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("event[%d]=%q want %q", i, got[i], w)
		}
	}
}
