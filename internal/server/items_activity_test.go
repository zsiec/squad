package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestItemActivity_ReturnsThreadMessages(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		testRepoID, int64(1700000000), "agent-a", "BUG-100", "say", "first message", "[]", "normal"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		testRepoID, int64(1700000060), "agent-b", "BUG-100", "progress", "50% done", "[]", "normal"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-100/activity?limit=10", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d events, want 2: %v", len(out), out)
	}
	// Newest first.
	if out[0]["kind"] != "progress" || out[0]["body"] != "50% done" {
		t.Errorf("first event=%v", out[0])
	}
	if out[1]["kind"] != "say" || out[1]["body"] != "first message" {
		t.Errorf("second event=%v", out[1])
	}
}

func TestItemActivity_LimitClamps(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()

	for i := 0; i < 5; i++ {
		if _, err := db.ExecContext(context.Background(),
			`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			testRepoID, int64(1700000000+i), "agent", "BUG-100", "say", "x", "[]", "normal"); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-100/activity?limit=2", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 2 {
		t.Fatalf("expected limit=2 honored, got %d", len(out))
	}
}

func TestItemActivity_EmptyForUnknownItem(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/items/NOPE-999/activity", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 0 {
		t.Fatalf("expected empty array for unknown item, got %d", len(out))
	}
}
