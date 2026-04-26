package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestItemsReject_HappyPath(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-600
title: investigate the flaky auth test we have
type: feat
status: captured
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-600", body)
	seedItem(t, s, path)

	rec := postJSON(t, s, "/api/items/FEAT-600/reject", map[string]any{
		"reason": "duplicate of FEAT-007",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be removed: %v", err)
	}
	var n int
	_ = s.db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-600").Scan(&n)
	if n != 0 {
		t.Fatalf("row should be deleted, count=%d", n)
	}
	log, err := os.ReadFile(filepath.Join(squadDir, "rejected.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(log), "duplicate of FEAT-007") {
		t.Fatalf("log missing reason: %s", log)
	}
	if !strings.Contains(string(log), "web") {
		t.Fatalf("log missing default by=web: %s", log)
	}
}

func TestItemsReject_ByOverride(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-601
title: investigate the flaky auth test we have
type: feat
status: captured
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-601", body)
	seedItem(t, s, path)

	rec := postJSON(t, s, "/api/items/FEAT-601/reject", map[string]any{
		"reason": "stale",
		"by":     "thomas@laptop",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	log, _ := os.ReadFile(filepath.Join(squadDir, "rejected.log"))
	if !strings.Contains(string(log), "thomas@laptop") {
		t.Fatalf("log missing by=thomas@laptop: %s", log)
	}
}

func TestItemsReject_MissingReasonReturns400(t *testing.T) {
	s, _ := newCreateServer(t)
	rec := postJSON(t, s, "/api/items/FEAT-600/reject", map[string]any{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsReject_EmptyReasonReturns400(t *testing.T) {
	s, _ := newCreateServer(t)
	rec := postJSON(t, s, "/api/items/FEAT-600/reject", map[string]any{
		"reason": "   ",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsReject_ClaimedReturns409(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-602
title: investigate the flaky auth test we have
type: feat
status: captured
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-602", body)
	seedItem(t, s, path)
	registerAgent(t, s.db, "agent-aaaa", "Alice")
	insertClaim(t, s.db, "agent-aaaa", "FEAT-602", "fix")

	rec := postJSON(t, s, "/api/items/FEAT-602/reject", map[string]any{
		"reason": "wrong",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
}

func TestItemsReject_UnknownIDReturns204(t *testing.T) {
	s, _ := newCreateServer(t)
	rec := postJSON(t, s, "/api/items/FEAT-999/reject", map[string]any{
		"reason": "doesn't matter",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
