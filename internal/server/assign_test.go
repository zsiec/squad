package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAssign_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-target", "Target")
	registerAgent(t, db, "agent-operator", "Op")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-target"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var holder, item, intent string
	if err := db.QueryRowContext(context.Background(),
		`SELECT agent_id, item_id, intent FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-100").Scan(&holder, &item, &intent); err != nil {
		t.Fatalf("claim row not found: %v", err)
	}
	if holder != "agent-target" || item != "BUG-100" {
		t.Fatalf("got holder=%q item=%q", holder, item)
	}
	if intent != "assigned by agent-operator" {
		t.Fatalf("expected intent to record operator; got %q", intent)
	}
}

func TestAssign_PostHonorsCallerSuppliedIntent(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-target", "Target")
	registerAgent(t, db, "agent-operator", "Op")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-target", "intent": "rush job"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var intent string
	if err := db.QueryRowContext(context.Background(),
		`SELECT intent FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-100").Scan(&intent); err != nil {
		t.Fatalf("query: %v", err)
	}
	if intent != "rush job" {
		t.Fatalf("expected supplied intent; got %q", intent)
	}
}

func TestAssign_PostMissingOperatorHeader(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-target", "Target")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-target"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAssign_PostMissingAgentID(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-operator", "Op")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-100").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("no claim row should exist; got %d", n)
	}
}

func TestAssign_PostUnknownTargetAgent(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-operator", "Op")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-ghost"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims WHERE repo_id=?`, testRepoID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("no claim row should exist for unknown target; got %d", n)
	}
}

func TestAssign_PostConflictWhenAlreadyClaimed(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	registerAgent(t, db, "agent-bbbb", "Bob")
	registerAgent(t, db, "agent-operator", "Op")
	insertClaim(t, db, "agent-aaaa", "BUG-100", "first")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-bbbb"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-100/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	var holder string
	if err := db.QueryRowContext(context.Background(),
		`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-100").Scan(&holder); err != nil {
		t.Fatalf("query: %v", err)
	}
	if holder != "agent-aaaa" {
		t.Fatalf("claim should still belong to agent-aaaa; got %q", holder)
	}
}

func TestAssign_PostBlockedByOpenItem(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-target", "Target")
	registerAgent(t, db, "agent-operator", "Op")

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeItem(t, itemsDir, "BUG-500-x.md", `---
id: BUG-500
title: blocked target
type: bug
priority: P2
area: server
status: ready
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
blocked-by: [BUG-499]
---

## Acceptance criteria

- [ ] First criterion
`)

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-target"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-500/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", rec.Code, rec.Body.String())
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-500").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("no claim row should exist for blocked item; got %d", n)
	}
}

func TestAssign_PostAlreadyDoneItem(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-target", "Target")
	registerAgent(t, db, "agent-operator", "Op")

	tmp := t.TempDir()
	doneDir := filepath.Join(tmp, "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeItem(t, doneDir, "BUG-600-x.md", `---
id: BUG-600
title: already done
type: bug
priority: P2
area: server
status: done
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
---

## Acceptance criteria

- [x] Done
`)

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-target"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-600/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-600").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("no claim row should exist for done item; got %d", n)
	}
}

func TestAssign_PostNotFoundForUnknownItem(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-target", "Target")
	registerAgent(t, db, "agent-operator", "Op")
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: "testdata"})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"agent_id": "agent-target"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-DOES-NOT-EXIST/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-operator")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims WHERE repo_id=?`, testRepoID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("no claim row should exist; got %d", n)
	}
}
