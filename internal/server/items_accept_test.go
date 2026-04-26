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

	"github.com/zsiec/squad/internal/items"
)

func writeCapturedItem(t *testing.T, squadDir, id, body string) string {
	t.Helper()
	itemsDir := filepath.Join(squadDir, "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(itemsDir, id+"-thing.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func seedItem(t *testing.T, s *Server, path string) items.Item {
	t.Helper()
	it, err := items.Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := items.Persist(context.Background(), s.db, s.cfg.RepoID, it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	return it
}

func TestItemsAccept_HappyPath(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-500
title: investigate the flaky auth test we have
type: feat
status: captured
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: web
captured_at: 100
---

## Acceptance criteria
- [ ] does the thing
`
	writeCapturedItem(t, squadDir, "FEAT-500", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-500-thing.md"))

	rec := postJSON(t, s, "/api/items/FEAT-500/accept", map[string]any{})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	var status, acceptedBy string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT status, COALESCE(accepted_by,'') FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-500",
	).Scan(&status, &acceptedBy); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "open" {
		t.Fatalf("status=%q want open", status)
	}
	if acceptedBy != "web" {
		t.Fatalf("accepted_by=%q want web", acceptedBy)
	}
}

func TestItemsAccept_AcceptedByOverride(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-501
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
- [ ] does the thing
`
	writeCapturedItem(t, squadDir, "FEAT-501", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-501-thing.md"))

	rec := postJSON(t, s, "/api/items/FEAT-501/accept", map[string]any{
		"accepted_by": "thomas@laptop",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var acceptedBy string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(accepted_by,'') FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-501",
	).Scan(&acceptedBy); err != nil {
		t.Fatalf("query: %v", err)
	}
	if acceptedBy != "thomas@laptop" {
		t.Fatalf("accepted_by=%q want thomas@laptop", acceptedBy)
	}
}

func TestItemsAccept_DoRFailReturns422(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-502
title: bad item
type: feat
status: captured
priority: P2
area: <fill-in>
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] x
`
	writeCapturedItem(t, squadDir, "FEAT-502", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-502-thing.md"))

	rec := postJSON(t, s, "/api/items/FEAT-502/accept", map[string]any{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	violations, ok := out["violations"].([]any)
	if !ok || len(violations) == 0 {
		t.Fatalf("violations missing or empty: %v", out)
	}
	first, _ := violations[0].(map[string]any)
	if first["rule"] == nil || first["message"] == nil {
		t.Fatalf("violation missing rule/message: %v", first)
	}
}

func TestItemsAccept_UnknownIDReturns404(t *testing.T) {
	s, _ := newCreateServer(t)
	rec := postJSON(t, s, "/api/items/FEAT-999/accept", map[string]any{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsAccept_AlreadyOpenReturns204(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-503
title: investigate the flaky auth test we have
type: feat
status: open
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
accepted_by: agent-X
accepted_at: 12345
---

## Acceptance criteria
- [ ] x
`
	writeCapturedItem(t, squadDir, "FEAT-503", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-503-thing.md"))

	rec := postJSON(t, s, "/api/items/FEAT-503/accept", map[string]any{})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsAccept_BlockedReturns409(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-504
title: blocked thing with many descriptive words
type: feat
status: blocked
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
	writeCapturedItem(t, squadDir, "FEAT-504", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-504-thing.md"))

	rec := postJSON(t, s, "/api/items/FEAT-504/accept", map[string]any{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestItemsAccept_EmptyBodyOK(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-505
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
- [ ] does the thing
`
	writeCapturedItem(t, squadDir, "FEAT-505", body)
	seedItem(t, s, filepath.Join(squadDir, "items", "FEAT-505-thing.md"))

	req := httptest.NewRequest(http.MethodPost, "/api/items/FEAT-505/accept", bytes.NewReader(nil))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}
