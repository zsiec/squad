package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBlocked_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	insertClaim(t, db, "agent-aaaa", "BUG-400", "fix")

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	_ = os.MkdirAll(itemsDir, 0o755)
	writeItem(t, itemsDir, "BUG-400-x.md", `---
id: BUG-400
title: blocked test
type: bug
priority: P2
area: server
status: ready
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
---

## Acceptance criteria

- [ ] First criterion
`)

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"reason": "waiting on infra"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-400/blocked", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// claim removed
	var holder string
	err := db.QueryRowContext(context.Background(),
		`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-400").Scan(&holder)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim should be gone, got holder=%q err=%v", holder, err)
	}
}

func TestBlocked_PostNoAgentHeader(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"reason": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-400/blocked", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestBlocked_PostEmptyReasonRejected(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	body, _ := json.Marshal(map[string]any{"reason": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-400/blocked", bytes.NewReader(body))
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
