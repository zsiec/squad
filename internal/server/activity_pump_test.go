package server

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE_ActivityPump_AgentEventInsert(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Give the SSE handler a tick to subscribe before inserting.
	time.Sleep(700 * time.Millisecond)

	insertAgentEvent(t, db, testRepoID, "agent-X", "PreToolUse", "Bash", "ls -la", 0)

	payload := waitSSEPayload(t, resp.Body, "agent_activity", 3*time.Second)
	if payload == nil {
		t.Fatal("did not see agent_activity event after agent_events insert")
	}
	if got := payload["agent_id"]; got != "agent-X" {
		t.Errorf("payload agent_id = %v; want agent-X", got)
	}
	if got := payload["source"]; got != "event" {
		t.Errorf("payload source = %v; want event", got)
	}
	if got := payload["tool"]; got != "Bash" {
		t.Errorf("payload tool = %v; want Bash", got)
	}
	if got := payload["target"]; got != "ls -la" {
		t.Errorf("payload target = %v; want ls -la", got)
	}
}

func TestSSE_ActivityPump_AttestationInsert(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(700 * time.Millisecond)

	insertAttestation(t, db, "agent-Y", "BUG-001", "test", 0, time.Now().Unix())

	payload := waitSSEPayload(t, resp.Body, "agent_activity", 3*time.Second)
	if payload == nil {
		t.Fatal("did not see agent_activity event after attestations insert")
	}
	if got := payload["source"]; got != "attestation" {
		t.Errorf("payload source = %v; want attestation", got)
	}
	if got := payload["agent_id"]; got != "agent-Y" {
		t.Errorf("payload agent_id = %v; want agent-Y", got)
	}
	if got := payload["item_id"]; got != "BUG-001" {
		t.Errorf("payload item_id = %v; want BUG-001", got)
	}
	if got := payload["attestation_kind"]; got != "test" {
		t.Errorf("payload attestation_kind = %v; want test", got)
	}
}

func TestSSE_ActivityPump_CommitInsert(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(700 * time.Millisecond)

	insertCommit(t, db, "agent-Z", "TASK-001", "abc1234", "feat: thing", time.Now().Unix())

	payload := waitSSEPayload(t, resp.Body, "agent_activity", 3*time.Second)
	if payload == nil {
		t.Fatal("did not see agent_activity event after commits insert")
	}
	if got := payload["source"]; got != "commit" {
		t.Errorf("payload source = %v; want commit", got)
	}
	if got := payload["agent_id"]; got != "agent-Z" {
		t.Errorf("payload agent_id = %v; want agent-Z", got)
	}
	if got := payload["sha"]; got != "abc1234" {
		t.Errorf("payload sha = %v; want abc1234", got)
	}
	if got := payload["subject"]; got != "feat: thing" {
		t.Errorf("payload subject = %v; want feat: thing", got)
	}
}

func TestSSE_ActivityPump_NoReplayAtBoot(t *testing.T) {
	db := newTestDB(t)
	// Pre-seed before the server starts: pump cursors / snapshots are
	// captured at start and pre-existing rows must NOT replay on the
	// first tick. Touches are seeded too because their snapshot-diff
	// drain is a separate path from the cursor-based commits/events.
	insertAgentEvent(t, db, testRepoID, "agent-prior", "PreToolUse", "Bash", "old", 0)
	insertTouch(t, db, testRepoID, "agent-prior", "BUG-prior", "internal/old.go", time.Now().Unix())

	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if waitSSEEvent(t, resp.Body, "agent_activity", 1500*time.Millisecond) {
		t.Fatal("expected no replay of pre-existing rows at boot, but saw an activity event")
	}
}

func insertAgentEvent(t *testing.T, db *sql.DB, repoID, agentID, eventKind, tool, target string, exit int) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO agent_events (repo_id, agent_id, ts, event_kind, tool, target, exit_code, duration_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		repoID, agentID, time.Now().Unix(), eventKind, tool, target, exit, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func insertTouch(t *testing.T, db *sql.DB, repoID, agentID, itemID, path string, ts int64) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO touches (repo_id, agent_id, item_id, path, started_at) VALUES (?, ?, ?, ?, ?)`,
		repoID, agentID, itemID, path, ts,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSSE_ActivityPump_TouchInsert(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(700 * time.Millisecond)

	insertTouch(t, db, testRepoID, "agent-T", "BUG-1", "internal/foo.go", time.Now().Unix())

	payload := waitSSEPayload(t, resp.Body, "agent_activity", 3*time.Second)
	if payload == nil {
		t.Fatal("did not see agent_activity event after touches insert")
	}
	if got := payload["source"]; got != "touch" {
		t.Errorf("payload source = %v; want touch", got)
	}
	if got := payload["kind"]; got != "touch" {
		t.Errorf("payload kind = %v; want touch", got)
	}
	if got := payload["agent_id"]; got != "agent-T" {
		t.Errorf("payload agent_id = %v; want agent-T", got)
	}
	if got := payload["path"]; got != "internal/foo.go" {
		t.Errorf("payload path = %v; want internal/foo.go", got)
	}
	if got := payload["item_id"]; got != "BUG-1" {
		t.Errorf("payload item_id = %v; want BUG-1", got)
	}
}

func TestSSE_ActivityPump_UntouchEmitsOnRelease(t *testing.T) {
	db := newTestDB(t)
	// Pre-seed the touch so the initial snapshot has it; the release after
	// subscription is the transition we want to see emitted.
	id := insertTouch(t, db, testRepoID, "agent-U", "BUG-2", "internal/bar.go", time.Now().Unix())

	s := New(db, testRepoID, Config{SquadDir: "testdata", LearningsRoot: "testdata/repo-root"})
	defer s.Close()

	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	time.Sleep(700 * time.Millisecond)

	if _, err := db.Exec(`UPDATE touches SET released_at = ? WHERE id = ?`, time.Now().Unix(), id); err != nil {
		t.Fatal(err)
	}

	payload := waitSSEPayload(t, resp.Body, "agent_activity", 3*time.Second)
	if payload == nil {
		t.Fatal("did not see agent_activity event after touches release")
	}
	if got := payload["source"]; got != "touch" {
		t.Errorf("payload source = %v; want touch", got)
	}
	if got := payload["kind"]; got != "untouch" {
		t.Errorf("payload kind = %v; want untouch", got)
	}
	if got := payload["agent_id"]; got != "agent-U" {
		t.Errorf("payload agent_id = %v; want agent-U", got)
	}
	if got := payload["path"]; got != "internal/bar.go" {
		t.Errorf("payload path = %v; want internal/bar.go", got)
	}
}

// waitSSEPayload reads the SSE stream until it sees `event: <kind>` and
// returns the decoded JSON payload from the following `data:` line, or nil
// if the deadline elapses first.
func waitSSEPayload(t *testing.T, body io.Reader, kind string, d time.Duration) map[string]any {
	t.Helper()
	scanner := bufio.NewScanner(body)
	deadline := time.Now().Add(d)
	for scanner.Scan() {
		if time.Now().After(deadline) {
			return nil
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "event: "+kind) {
			continue
		}
		// The next non-empty line is `data: {...}`.
		for scanner.Scan() {
			data := scanner.Text()
			if strings.HasPrefix(data, "data: ") {
				var env struct {
					Kind    string         `json:"Kind"`
					Payload map[string]any `json:"Payload"`
				}
				if err := json.Unmarshal([]byte(strings.TrimPrefix(data, "data: ")), &env); err != nil {
					return nil
				}
				return env.Payload
			}
			if data != "" {
				break
			}
		}
	}
	return nil
}
