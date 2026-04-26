package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTouch_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	insertClaim(t, db, "agent-aaaa", "BUG-600", "fix")

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-600/touch", nil)
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// last_touch should be advanced past the inserted 0 baseline
	var lastTouch int64
	if err := db.QueryRowContext(context.Background(),
		`SELECT last_touch FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-600").Scan(&lastTouch); err != nil {
		t.Fatal(err)
	}
	if lastTouch == 0 {
		t.Fatalf("expected last_touch advanced, got %d", lastTouch)
	}
}

func TestTouch_PostNoAgentHeader(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-600/touch", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestTouch_PostNoActiveClaimReturns404(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-600/touch", nil)
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
