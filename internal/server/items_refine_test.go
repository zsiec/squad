package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestItemsRefine_HappyPath(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-800
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

## Problem

Auth test is flaky.

## Acceptance criteria
- [ ] does the thing
`
	path := writeCapturedItem(t, squadDir, "FEAT-800", body)
	seedItem(t, s, path)

	rec := postJSON(t, s, "/api/items/FEAT-800/refine", map[string]any{
		"comments": "tighten the acceptance criteria",
	})
	if rec.Code != 204 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "## Reviewer feedback") {
		t.Fatalf("body missing reviewer feedback section:\n%s", raw)
	}
	if !strings.Contains(string(raw), "tighten the acceptance criteria") {
		t.Fatalf("body missing comment text:\n%s", raw)
	}

	var status string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT status FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-800",
	).Scan(&status); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "needs-refinement" {
		t.Fatalf("status=%q want needs-refinement", status)
	}
}

func TestItemsRefine_EmptyCommentsReturns422(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-801
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

## Problem
x

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-801", body)
	seedItem(t, s, path)

	rec := postJSON(t, s, "/api/items/FEAT-801/refine", map[string]any{
		"comments": "   ",
	})
	if rec.Code != 422 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsRefine_WrongStatusReturns422(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-802
title: investigate the flaky auth test we have
type: feat
status: open
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Problem
x

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-802", body)
	seedItem(t, s, path)

	rec := postJSON(t, s, "/api/items/FEAT-802/refine", map[string]any{
		"comments": "needs more detail",
	})
	if rec.Code != 422 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSSE_InboxChanged_OnRefine(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-803
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

## Problem
x

## Acceptance criteria
- [ ] x
`
	path := writeCapturedItem(t, squadDir, "FEAT-803", body)
	seedItem(t, s, path)

	sub := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(sub)

	rec := postJSON(t, s, "/api/items/FEAT-803/refine", map[string]any{
		"comments": "split out the retry logic",
	})
	if rec.Code != 204 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	ev, ok := waitForKind(t, sub, "inbox_changed", 2*time.Second)
	if !ok {
		t.Fatal("did not see inbox_changed event after POST /api/items/{id}/refine")
	}
	if ev.Payload["action"] != "refine" {
		t.Fatalf("action=%v want refine", ev.Payload["action"])
	}
	if ev.Payload["item_id"] != "FEAT-803" {
		t.Fatalf("item_id=%v want FEAT-803", ev.Payload["item_id"])
	}
}
