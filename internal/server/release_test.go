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
)

func TestRelease_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	insertClaim(t, db, "agent-aaaa", "BUG-100", "fix")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"outcome": "released"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/release", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// claim row gone
	var holder string
	err := db.QueryRowContext(context.Background(),
		`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-100").Scan(&holder)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim row should be gone, got holder=%q err=%v", holder, err)
	}
}

func TestRelease_PostNoAgentHeader(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/release", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestRelease_PostNotClaimed(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/release", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRelease_PostNotYours(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	registerAgent(t, db, "agent-bbbb", "Bob")
	insertClaim(t, db, "agent-aaaa", "BUG-100", "alice's claim")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/release", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Squad-Agent", "agent-bbbb")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d body=%s", rec.Code, rec.Body.String())
	}
}
