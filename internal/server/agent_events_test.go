package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentEvents_404WhenAgentMissing(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-ghost/events", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentEvents_EmptyForRegisteredAgent(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-empty", "Empty")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-empty/events", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out struct {
		Events     []any `json:"events"`
		NextCursor *int  `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Events) != 0 {
		t.Errorf("events=%v want []", out.Events)
	}
	if out.NextCursor != nil {
		t.Errorf("next_cursor=%v want nil", out.NextCursor)
	}
}

func TestAgentEvents_ReturnsRowsDescByDefault(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	insertEvent(t, db, "agent-a", 100, "PreToolUse", "Bash", "ls", 0, 12)
	insertEvent(t, db, "agent-a", 200, "PostToolUse", "Bash", "ls", 0, 34)
	insertEvent(t, db, "agent-b", 150, "PreToolUse", "Read", "x", 0, 0)

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/events", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Events []map[string]any `json:"events"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Events) != 2 {
		t.Fatalf("len=%d want 2 (cross-agent leak?)", len(out.Events))
	}
	if int64(out.Events[0]["ts"].(float64)) != 200 || int64(out.Events[1]["ts"].(float64)) != 100 {
		t.Errorf("default order should be ts DESC; got %v", out.Events)
	}
	if out.Events[0]["tool"] != "Bash" || out.Events[0]["target"] != "ls" {
		t.Errorf("row missing fields: %+v", out.Events[0])
	}
}

func TestAgentEvents_SinceFiltersAndOrdersAsc(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	insertEvent(t, db, "agent-a", 100, "PreToolUse", "Bash", "ls", 0, 0)
	insertEvent(t, db, "agent-a", 200, "PostToolUse", "Bash", "ls", 0, 0)
	insertEvent(t, db, "agent-a", 300, "PreToolUse", "Read", "f", 0, 0)

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/events?since=200", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var out struct {
		Events []map[string]any `json:"events"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Events) != 2 {
		t.Fatalf("len=%d want 2 (since cuts ts<200)", len(out.Events))
	}
	if int64(out.Events[0]["ts"].(float64)) != 200 || int64(out.Events[1]["ts"].(float64)) != 300 {
		t.Errorf("since order should be ts ASC; got %v", out.Events)
	}
}

func TestAgentEvents_LimitCaps(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	for i := int64(1); i <= 10; i++ {
		insertEvent(t, db, "agent-a", i, "PreToolUse", "Bash", "x", 0, 0)
	}

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/events?limit=3", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var out struct {
		Events []map[string]any `json:"events"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Events) != 3 {
		t.Fatalf("len=%d want 3", len(out.Events))
	}
}

func TestAgentEvents_BadParamsReturn400(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	for _, qs := range []string{"?since=oops", "?limit=abc", "?limit=0", "?limit=9999"} {
		req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/events"+qs, nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("qs=%q code=%d want 400", qs, rec.Code)
		}
	}
}

func TestAgentTimeline_404WhenAgentMissing(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-ghost/timeline", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestAgentTimeline_UnionsSourcesByAgent(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	registerAgent(t, db, "agent-b", "B")

	insertMessage(t, db, "agent-a", 100, "global", "say", "hello")
	insertMessage(t, db, "agent-a", 110, "BUG-1", "claim", "claimed BUG-1")
	insertClaimHistoryRow(t, db, "agent-a", "BUG-1", 90, 200, "done")
	insertCommit(t, db, "agent-a", "BUG-1", "abcd123", "fix it", 150)
	insertAttestation(t, db, "agent-a", "BUG-1", "test", 0, 160)
	insertEvent(t, db, "agent-a", 105, "PreToolUse", "Bash", "ls", 0, 0)

	// Cross-agent leak guard
	insertMessage(t, db, "agent-b", 120, "global", "say", "other")
	insertEvent(t, db, "agent-b", 125, "PreToolUse", "Read", "x", 0, 0)

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/timeline", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Timeline []map[string]any `json:"timeline"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Timeline) != 6 {
		t.Fatalf("len=%d want 6 (chat×2, claim_history, commit, attestation, event)\n%+v", len(out.Timeline), out.Timeline)
	}
	kinds := map[string]int{}
	for _, row := range out.Timeline {
		kinds[row["kind"].(string)]++
		if row["agent_id"] != "agent-a" {
			t.Errorf("cross-agent leak: %+v", row)
		}
	}
	want := map[string]int{"chat": 2, "done": 1, "commit": 1, "attestation": 1, "event": 1}
	for k, v := range want {
		if kinds[k] != v {
			t.Errorf("kind %q count=%d want %d (got %v)", k, kinds[k], v, kinds)
		}
	}
}

func TestAgentTimeline_ActiveClaimsAppearAsClaimKind(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	insertClaim(t, db, "agent-a", "BUG-1", "intent here")

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/timeline", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Timeline []map[string]any `json:"timeline"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Timeline) != 1 {
		t.Fatalf("len=%d want 1: %+v", len(out.Timeline), out.Timeline)
	}
	if out.Timeline[0]["kind"] != "claim" || out.Timeline[0]["item_id"] != "BUG-1" || out.Timeline[0]["intent"] != "intent here" {
		t.Errorf("active claim row missing fields: %+v", out.Timeline[0])
	}
}

func TestAgentTimeline_DescByDefaultAndAscWithSince(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	insertMessage(t, db, "agent-a", 100, "global", "say", "first")
	insertEvent(t, db, "agent-a", 200, "PreToolUse", "Bash", "x", 0, 0)
	insertCommit(t, db, "agent-a", "BUG-1", "sha", "subj", 300)

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/timeline", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var out struct {
		Timeline []map[string]any `json:"timeline"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Timeline) != 3 {
		t.Fatalf("len=%d want 3", len(out.Timeline))
	}
	if int64(out.Timeline[0]["ts"].(float64)) != 300 {
		t.Errorf("default DESC: got first ts=%v want 300", out.Timeline[0]["ts"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/timeline?since=200", nil)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out.Timeline) != 2 {
		t.Fatalf("since=200 len=%d want 2", len(out.Timeline))
	}
	if int64(out.Timeline[0]["ts"].(float64)) != 200 || int64(out.Timeline[1]["ts"].(float64)) != 300 {
		t.Errorf("since order ASC; got %v / %v", out.Timeline[0]["ts"], out.Timeline[1]["ts"])
	}
}

func TestAgentTimeline_TsTieDeterministic(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	insertMessage(t, db, "agent-a", 100, "global", "say", "msg")
	insertCommit(t, db, "agent-a", "BUG-1", "sha", "subj", 100)
	insertEvent(t, db, "agent-a", 100, "PreToolUse", "Bash", "x", 0, 0)
	insertAttestation(t, db, "agent-a", "BUG-1", "test", 0, 100)

	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	var first, second []map[string]any
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/timeline?since=0", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		var out struct {
			Timeline []map[string]any `json:"timeline"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&out)
		if i == 0 {
			first = out.Timeline
		} else {
			second = out.Timeline
		}
	}
	if len(first) != 4 || len(second) != 4 {
		t.Fatalf("want 4/4, got %d/%d", len(first), len(second))
	}
	for i := range first {
		if first[i]["kind"] != second[i]["kind"] {
			t.Errorf("non-deterministic ordering at i=%d: %v vs %v", i, first[i]["kind"], second[i]["kind"])
		}
	}
}

func TestAgentTimeline_BadParams400(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-a", "A")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()

	for _, qs := range []string{"?since=nope", "?limit=-5", "?limit=99999"} {
		req := httptest.NewRequest(http.MethodGet, "/api/agents/agent-a/timeline"+qs, nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("qs=%q code=%d want 400", qs, rec.Code)
		}
	}
}

func insertEvent(t *testing.T, db *sql.DB, agentID string, ts int64, kind, tool, target string, exit int, dur int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agent_events (repo_id, agent_id, session_id, ts, event_kind, tool, target, exit_code, duration_ms)
		VALUES (?, ?, '', ?, ?, ?, ?, ?, ?)
	`, testRepoID, agentID, ts, kind, tool, target, exit, dur); err != nil {
		t.Fatalf("insert agent_events: %v", err)
	}
}

func insertMessage(t *testing.T, db *sql.DB, agentID string, ts int64, thread, kind, body string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, ?, ?, ?, '[]', 'normal')
	`, testRepoID, ts, agentID, thread, kind, body); err != nil {
		t.Fatalf("insert messages: %v", err)
	}
}

func insertClaimHistoryRow(t *testing.T, db *sql.DB, agentID, itemID string, claimedAt, releasedAt int64, outcome string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		VALUES (?, ?, ?, ?, ?, ?)
	`, testRepoID, itemID, agentID, claimedAt, releasedAt, outcome); err != nil {
		t.Fatalf("insert claim_history: %v", err)
	}
}

func insertCommit(t *testing.T, db *sql.DB, agentID, itemID, sha, subject string, ts int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO commits (repo_id, item_id, sha, subject, ts, agent_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, testRepoID, itemID, sha, subject, ts, agentID); err != nil {
		t.Fatalf("insert commits: %v", err)
	}
}

func insertAttestation(t *testing.T, db *sql.DB, agentID, itemID, kind string, exit int, ts int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO attestations (item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, repo_id)
		VALUES (?, ?, '', ?, ?, '', ?, ?, ?)
	`, itemID, kind, exit, sha256Hex(itemID, ts), ts, agentID, testRepoID); err != nil {
		t.Fatalf("insert attestations: %v", err)
	}
}

func sha256Hex(seed string, ts int64) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = 'a'
	}
	for i, b := range []byte(seed) {
		if i >= 32 {
			break
		}
		out[i] = byte('a' + (b+byte(ts))%26)
	}
	return string(out)
}
