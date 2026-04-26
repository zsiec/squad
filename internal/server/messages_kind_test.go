package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessages_PostWithKind(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"thread": "global", "body": "is X true?", "kind": "ask"})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var kind string
	if err := db.QueryRowContext(context.Background(),
		`SELECT kind FROM messages WHERE repo_id=? AND agent_id=? ORDER BY id DESC LIMIT 1`,
		testRepoID, "agent-aaaa").Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "ask" {
		t.Fatalf("got kind=%q, want ask", kind)
	}
}

func TestMessages_PostKindDefaultsToSay(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"thread": "global", "body": "ping"})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var kind string
	if err := db.QueryRowContext(context.Background(),
		`SELECT kind FROM messages WHERE repo_id=? AND agent_id=? ORDER BY id DESC LIMIT 1`,
		testRepoID, "agent-aaaa").Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "say" {
		t.Fatalf("got kind=%q, want say", kind)
	}
}

func TestMessages_PostUnknownKindRejected(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"thread": "global", "body": "x", "kind": "not-a-kind"})
	req := httptest.NewRequest(http.MethodPost, "/api/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
