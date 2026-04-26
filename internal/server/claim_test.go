package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClaim_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"intent": "fix the bug"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// DB row should exist
	var holder, intent string
	if err := db.QueryRowContext(context.Background(),
		`SELECT agent_id, intent FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-100").Scan(&holder, &intent); err != nil {
		t.Fatalf("claim row not found: %v", err)
	}
	if holder != "agent-aaaa" || intent != "fix the bug" {
		t.Fatalf("got holder=%q intent=%q", holder, intent)
	}
}

func TestClaim_PostNoAgentHeader(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/claim", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestClaim_PostConflictWhenAlreadyClaimed(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	registerAgent(t, db, "agent-bbbb", "Bob")
	insertClaim(t, db, "agent-aaaa", "BUG-100", "first")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"intent": "second"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-bbbb")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}
