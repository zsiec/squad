package server

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// waitForKind drains bus events until one with the requested kind arrives or
// the deadline expires. Returns the matching event or zero value + false.
func waitForKind(t *testing.T, sub chan chat.Event, kind string, d time.Duration) (chat.Event, bool) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case e, ok := <-sub:
			if !ok {
				return chat.Event{}, false
			}
			if e.Kind == kind {
				return e, true
			}
		case <-deadline:
			return chat.Event{}, false
		}
	}
}

func TestSSE_InboxChanged_OnCreate(t *testing.T) {
	s, _ := newCreateServer(t)
	sub := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(sub)

	rec := postJSON(t, s, "/api/items", map[string]any{
		"type":  "feat",
		"title": "publish a sse event when capturing a new item",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	ev, ok := waitForKind(t, sub, "inbox_changed", 2*time.Second)
	if !ok {
		t.Fatal("did not see inbox_changed event after POST /api/items")
	}
	if ev.Payload["action"] != "captured" {
		t.Fatalf("action=%v want captured", ev.Payload["action"])
	}
	id, _ := ev.Payload["item_id"].(string)
	if id == "" {
		t.Fatalf("item_id missing in payload: %v", ev.Payload)
	}
}

func TestSSE_InboxChanged_OnAccept(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-700
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
	writeCapturedItem(t, squadDir, "FEAT-700", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-700-thing.md"))

	sub := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(sub)

	rec := postJSON(t, s, "/api/items/FEAT-700/accept", map[string]any{})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	ev, ok := waitForKind(t, sub, "inbox_changed", 2*time.Second)
	if !ok {
		t.Fatal("did not see inbox_changed event after POST /api/items/{id}/accept")
	}
	if ev.Payload["action"] != "accepted" {
		t.Fatalf("action=%v want accepted", ev.Payload["action"])
	}
	if ev.Payload["item_id"] != "FEAT-700" {
		t.Fatalf("item_id=%v want FEAT-700", ev.Payload["item_id"])
	}
}

func TestSSE_InboxChanged_OnReject(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-701
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
	path := writeCapturedItem(t, squadDir, "FEAT-701", body)
	seedItem(t, s, path)

	sub := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(sub)

	rec := postJSON(t, s, "/api/items/FEAT-701/reject", map[string]any{
		"reason": "duplicate of FEAT-007",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	ev, ok := waitForKind(t, sub, "inbox_changed", 2*time.Second)
	if !ok {
		t.Fatal("did not see inbox_changed event after POST /api/items/{id}/reject")
	}
	if ev.Payload["action"] != "rejected" {
		t.Fatalf("action=%v want rejected", ev.Payload["action"])
	}
	if ev.Payload["item_id"] != "FEAT-701" {
		t.Fatalf("item_id=%v want FEAT-701", ev.Payload["item_id"])
	}
}
