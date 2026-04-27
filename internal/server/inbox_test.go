package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

func seedCaptured(t *testing.T, dir, id, title, capturedBy string, capturedAt int64, dorPass bool) string {
	t.Helper()
	itemsDir := filepath.Join(dir, "items")
	area := "server"
	if !dorPass {
		area = "<fill-in>"
	}
	body := fmt.Sprintf(`---
id: %s
title: %s
type: bug
priority: P2
area: %s
status: captured
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
captured_by: %s
captured_at: %d
parent_spec: auth-rotation
---

## Acceptance criteria

- [ ] %s
`, id, title, area, capturedBy, capturedAt, title)
	return writeItem(t, itemsDir, id+"-x.md", body)
}

func seedOpen(t *testing.T, dir, id, title string) string {
	t.Helper()
	itemsDir := filepath.Join(dir, "items")
	body := fmt.Sprintf(`---
id: %s
title: %s
type: bug
priority: P2
area: server
status: open
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
---

## Acceptance criteria

- [ ] %s
`, id, title, title)
	return writeItem(t, itemsDir, id+"-x.md", body)
}

func TestInbox_Empty(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: t.TempDir()})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/inbox", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty array, got %v", out)
	}
}

func TestInbox_OrdersByCapturedAtAsc(t *testing.T) {
	db := newTestDB(t)
	tmp := t.TempDir()

	pathOld := seedCaptured(t, tmp, "BUG-401", "older capture", "agent-old", 1700000010, true)
	pathNew := seedCaptured(t, tmp, "BUG-402", "newer capture", "agent-new", 1700000050, false)
	pathOpen := seedOpen(t, tmp, "BUG-403", "already open")

	for _, p := range []string{pathOld, pathNew, pathOpen} {
		it, err := items.Parse(p)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		if err := items.Persist(context.Background(), db, testRepoID, it, false); err != nil {
			t.Fatalf("persist %s: %v", p, err)
		}
	}

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/inbox", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 captured items, got %d: %v", len(out), out)
	}
	if out[0]["id"] != "BUG-401" || out[1]["id"] != "BUG-402" {
		t.Fatalf("expected ascending captured_at order [BUG-401, BUG-402], got %v / %v",
			out[0]["id"], out[1]["id"])
	}
	if _, ok := out[0]["dor_pass"].(bool); !ok {
		t.Fatalf("dor_pass should be bool, got %T", out[0]["dor_pass"])
	}
	if _, ok := out[1]["dor_pass"].(bool); !ok {
		t.Fatalf("dor_pass should be bool, got %T", out[1]["dor_pass"])
	}
	if out[1]["dor_pass"].(bool) {
		t.Fatalf("BUG-402 (area=<fill-in>) should have dor_pass=false, got true")
	}
}

func seedAutoRefined(t *testing.T, dir, id, title string, refinedAt int64) string {
	t.Helper()
	itemsDir := filepath.Join(dir, "items")
	body := fmt.Sprintf(`---
id: %s
title: %s
type: bug
priority: P2
area: server
status: captured
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
captured_by: agent-x
captured_at: 1700000300
auto_refined_at: %d
auto_refined_by: claude
---

## Acceptance criteria

- [ ] %s
`, id, title, refinedAt, title)
	return writeItem(t, itemsDir, id+"-x.md", body)
}

func TestInbox_SurfacesAutoRefinedFields(t *testing.T) {
	db := newTestDB(t)
	tmp := t.TempDir()

	plainPath := seedCaptured(t, tmp, "BUG-601",
		"this title is intentionally longer than five words", "agent-a", 1700000100, true)
	autoPath := seedAutoRefined(t, tmp, "BUG-602",
		"this title is intentionally longer than five words", 1700000400)

	for _, p := range []string{plainPath, autoPath} {
		it, err := items.Parse(p)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		if err := items.Persist(context.Background(), db, testRepoID, it, false); err != nil {
			t.Fatalf("persist %s: %v", p, err)
		}
	}

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/inbox", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := map[string]map[string]any{}
	for _, e := range out {
		byID[e["id"].(string)] = e
	}

	plain := byID["BUG-601"]
	if _, ok := plain["auto_refined_at"]; ok {
		t.Errorf("untouched item must omit auto_refined_at; got %v", plain["auto_refined_at"])
	}
	if _, ok := plain["auto_refined_by"]; ok {
		t.Errorf("untouched item must omit auto_refined_by; got %v", plain["auto_refined_by"])
	}

	auto := byID["BUG-602"]
	if got := int64(auto["auto_refined_at"].(float64)); got != 1700000400 {
		t.Errorf("auto_refined_at=%d want 1700000400", got)
	}
	if auto["auto_refined_by"] != "claude" {
		t.Errorf("auto_refined_by=%v want claude", auto["auto_refined_by"])
	}
}

func TestInbox_DoRPassReflectsCheck(t *testing.T) {
	db := newTestDB(t)
	tmp := t.TempDir()

	passPath := seedCaptured(t, tmp, "BUG-501",
		"this title is intentionally longer than five words", "agent-a", 1700000100, true)
	failPath := seedCaptured(t, tmp, "BUG-502",
		"short title", "agent-b", 1700000200, false)

	for _, p := range []string{passPath, failPath} {
		it, err := items.Parse(p)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		if err := items.Persist(context.Background(), db, testRepoID, it, false); err != nil {
			t.Fatalf("persist %s: %v", p, err)
		}
	}

	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	defer s.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/inbox", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(out), out)
	}
	byID := map[string]map[string]any{}
	for _, e := range out {
		byID[e["id"].(string)] = e
	}
	if got := byID["BUG-501"]["dor_pass"]; got != true {
		t.Fatalf("BUG-501 dor_pass=%v, want true", got)
	}
	if got := byID["BUG-502"]["dor_pass"]; got != false {
		t.Fatalf("BUG-502 dor_pass=%v, want false", got)
	}
	pass := byID["BUG-501"]
	if pass["title"] != "this title is intentionally longer than five words" {
		t.Fatalf("title=%v", pass["title"])
	}
	if pass["captured_by"] != "agent-a" {
		t.Fatalf("captured_by=%v", pass["captured_by"])
	}
	if int64(pass["captured_at"].(float64)) != 1700000100 {
		t.Fatalf("captured_at=%v", pass["captured_at"])
	}
	if pass["parent_spec"] != "auth-rotation" {
		t.Fatalf("parent_spec=%v", pass["parent_spec"])
	}
	if _, ok := pass["path"].(string); !ok || pass["path"] == "" {
		t.Fatalf("path missing or empty: %v", pass["path"])
	}
}
