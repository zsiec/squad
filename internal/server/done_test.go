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
	"strings"
	"testing"
)

func writeItem(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	return p
}

func TestDone_PostHappyPath(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	insertClaim(t, db, "agent-aaaa", "BUG-200", "fix")

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	_ = os.MkdirAll(doneDir, 0o755)
	itemPath := writeItem(t, itemsDir, "BUG-200-x.md", `---
id: BUG-200
title: simple
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

- [x] First criterion
`)

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-200/done", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	// claim row removed
	var holder string
	err := db.QueryRowContext(context.Background(),
		`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "BUG-200").Scan(&holder)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim should be gone, got holder=%q err=%v", holder, err)
	}
	// item file moved to doneDir
	if _, err := os.Stat(itemPath); !os.IsNotExist(err) {
		t.Fatalf("expected source file removed, got err=%v", err)
	}
	entries, _ := os.ReadDir(doneDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in done/, got %d", len(entries))
	}
}

func TestDone_PostNoAgentHeader(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/items/BUG-200/done", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestDone_PostEvidenceMissingReturns412(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	insertClaim(t, db, "agent-aaaa", "FEAT-300", "fix")

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	_ = os.MkdirAll(doneDir, 0o755)
	writeItem(t, itemsDir, "FEAT-300-evi.md", `---
id: FEAT-300
title: needs evidence
type: feature
priority: P2
area: server
status: ready
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test]
---

## Acceptance criteria

- [x] First criterion
`)

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/items/FEAT-300/done", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "evidence") {
		t.Fatalf("expected evidence in body, got %s", rec.Body.String())
	}

	// Claim should still exist (we did NOT close)
	var holder string
	if err := db.QueryRowContext(context.Background(),
		`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-300").Scan(&holder); err != nil {
		t.Fatalf("claim should still exist: %v", err)
	}
}

func TestDone_PostEvidenceForcedSucceeds(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-aaaa", "Alice")
	insertClaim(t, db, "agent-aaaa", "FEAT-301", "fix")

	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	doneDir := filepath.Join(tmp, "done")
	_ = os.MkdirAll(doneDir, 0o755)
	writeItem(t, itemsDir, "FEAT-301-evi.md", `---
id: FEAT-301
title: needs evidence
type: feature
priority: P2
area: server
status: ready
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test]
---

## Acceptance criteria

- [x] First criterion
`)

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	body, _ := json.Marshal(map[string]any{"evidence_force": true})
	req := httptest.NewRequest(http.MethodPost, "/api/items/FEAT-301/done", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-aaaa")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}
