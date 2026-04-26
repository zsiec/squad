package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func postWithAgent(t *testing.T, s *Server, path, agent string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(nil))
	if agent != "" {
		req.Header.Set("X-Squad-Agent", agent)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestItemsRecapture_HappyPath(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-900
title: investigate the flaky auth test we have
type: feat
status: needs-refinement
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Reviewer feedback
tighten the acceptance criteria

## Problem

Auth test is flaky.

## Acceptance criteria
- [ ] does the thing
`
	path := writeCapturedItem(t, squadDir, "FEAT-900", body)
	seedItem(t, s, path)
	registerAgent(t, s.db, "agent-aaaa", "Alice")
	insertClaim(t, s.db, "agent-aaaa", "FEAT-900", "refine pass")

	rec := postWithAgent(t, s, "/api/items/FEAT-900/recapture", "agent-aaaa")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "## Reviewer feedback") {
		t.Fatalf("reviewer feedback should be gone:\n%s", raw)
	}
	if !strings.Contains(string(raw), "## Refinement history") {
		t.Fatalf("history section missing:\n%s", raw)
	}
	if !strings.Contains(string(raw), "### Round 1") {
		t.Fatalf("round 1 entry missing:\n%s", raw)
	}

	var status string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT status FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-900",
	).Scan(&status); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "captured" {
		t.Fatalf("status=%q want captured", status)
	}

	var holder string
	err = s.db.QueryRowContext(context.Background(),
		`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-900",
	).Scan(&holder)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim row should be gone, got holder=%q err=%v", holder, err)
	}
}

func TestItemsRecapture_RejectsWithoutClaim(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-901
title: investigate the flaky auth test we have
type: feat
status: needs-refinement
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Reviewer feedback
tighten

## Problem
x

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-901", body)
	seedItem(t, s, path)
	registerAgent(t, s.db, "agent-aaaa", "Alice")

	rec := postWithAgent(t, s, "/api/items/FEAT-901/recapture", "agent-aaaa")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%s want 403", rec.Code, rec.Body.String())
	}
}

func TestItemsRecapture_AppendsRound2(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-902
title: investigate the flaky auth test we have
type: feat
status: needs-refinement
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Reviewer feedback
round 2 ask

## Refinement history
### Round 1 — 2026-04-25
first ask

## Problem
x

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-902", body)
	seedItem(t, s, path)
	registerAgent(t, s.db, "agent-aaaa", "Alice")
	insertClaim(t, s.db, "agent-aaaa", "FEAT-902", "round 2")

	rec := postWithAgent(t, s, "/api/items/FEAT-902/recapture", "agent-aaaa")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "### Round 1 — 2026-04-25") {
		t.Fatalf("round 1 missing after recapture:\n%s", raw)
	}
	if !strings.Contains(string(raw), "### Round 2") {
		t.Fatalf("round 2 missing after recapture:\n%s", raw)
	}
	if strings.Contains(string(raw), "## Reviewer feedback") {
		t.Fatalf("reviewer feedback should be gone:\n%s", raw)
	}
}

func TestSSE_InboxChanged_OnRecapture(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-903
title: investigate the flaky auth test we have
type: feat
status: needs-refinement
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Reviewer feedback
tighten

## Problem
x

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-903", body)
	seedItem(t, s, path)
	registerAgent(t, s.db, "agent-aaaa", "Alice")
	insertClaim(t, s.db, "agent-aaaa", "FEAT-903", "refine pass")

	sub := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(sub)

	rec := postWithAgent(t, s, "/api/items/FEAT-903/recapture", "agent-aaaa")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	ev, ok := waitForKind(t, sub, "inbox_changed", 2*time.Second)
	if !ok {
		t.Fatal("did not see inbox_changed event after POST /api/items/{id}/recapture")
	}
	if ev.Payload["action"] != "recapture" {
		t.Fatalf("action=%v want recapture", ev.Payload["action"])
	}
	if ev.Payload["item_id"] != "FEAT-903" {
		t.Fatalf("item_id=%v want FEAT-903", ev.Payload["item_id"])
	}
}
