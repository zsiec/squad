package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zsiec/squad/internal/claims"
)

func TestHandoff_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	registerAgent(t, db, "agent-bbbb", "Bob")
	insertClaim(t, db, "agent-aaaa", "BUG-500", "fix")

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"to": "agent-bbbb", "summary": "wrapping up; bob takes over"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-500/handoff", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// claim should be released by reassign
	holder, err := claims.HolderOf(context.Background(), db, testRepoID, "BUG-500")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim should be released after reassign, got holder=%q err=%v", holder, err)
	}

	// a handoff message should exist
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM messages WHERE repo_id=? AND kind='handoff' AND agent_id=?`,
		testRepoID, "agent-aaaa").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected at least one handoff message")
	}
}

func TestHandoff_PostNoAgentHeader(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"to": "agent-bbbb"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-500/handoff", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandoff_PostEmptyToRejected(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"to": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-500/handoff", bytes.NewReader(body))
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
