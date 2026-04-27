package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// End-to-end loop for the inbox-refinement epic:
// captured → refine → list → claim → recapture → inbox.
func TestIntegration_RefineRoundTrip(t *testing.T) {
	s, tmp := newCreateServer(t)
	squadDir := filepath.Join(tmp, ".squad")
	body := `---
id: FEAT-950
title: build the new dashboard widget
type: feat
status: captured
priority: P2
area: web-ui
estimate: 2h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Problem

Need a dashboard widget.

## Acceptance criteria
- [ ] the rule does the thing as specified
`
	path := writeCapturedItem(t, squadDir, "FEAT-950", body)
	seedItem(t, s, path)
	registerAgent(t, s.db, "agent-zzzz", "Zoe")

	// 1. refine — captured → needs-refinement
	const refineComment = "tighten the AC; the widget contract is undefined"
	if rec := postJSON(t, s, "/api/items/FEAT-950/refine", map[string]any{
		"comments": refineComment,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("refine code=%d body=%s", rec.Code, rec.Body.String())
	}

	post1, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item after refine: %v", err)
	}
	if !strings.Contains(string(post1), "## Reviewer feedback") {
		t.Fatalf("item missing `## Reviewer feedback` section after refine:\n%s", post1)
	}
	if !strings.Contains(string(post1), refineComment) {
		t.Fatalf("item missing reviewer comment text after refine:\n%s", post1)
	}

	// 2. /api/refine lists it
	rec := getReq(t, s, "/api/refine")
	if rec.Code != http.StatusOK {
		t.Fatalf("refine list code=%d body=%s", rec.Code, rec.Body.String())
	}
	var refineList []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &refineList); err != nil {
		t.Fatalf("decode refine list: %v", err)
	}
	if !containsID(refineList, "FEAT-950") {
		t.Fatalf("FEAT-950 missing from /api/refine: %v", refineList)
	}

	// 3. claim — refining agent picks it up
	claimBody, _ := json.Marshal(map[string]any{"intent": "refine pass"})
	req := httptest.NewRequest(http.MethodPost, "/api/items/FEAT-950/claim", bytes.NewReader(claimBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Squad-Agent", "agent-zzzz")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("claim code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 4. recapture — needs-refinement → captured, claim released, history appended
	if rec := postWithAgent(t, s, "/api/items/FEAT-950/recapture", "agent-zzzz"); rec.Code != http.StatusNoContent {
		t.Fatalf("recapture code=%d body=%s", rec.Code, rec.Body.String())
	}

	post4, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item after recapture: %v", err)
	}
	if !strings.Contains(string(post4), "## Refinement history") {
		t.Fatalf("item missing `## Refinement history` after recapture:\n%s", post4)
	}
	if !strings.Contains(string(post4), "### Round 1") {
		t.Fatalf("item missing `### Round 1` after recapture:\n%s", post4)
	}
	if strings.Contains(string(post4), "## Reviewer feedback") {
		t.Fatalf("item still has `## Reviewer feedback` after recapture (should be moved to history):\n%s", post4)
	}

	// 5. /api/inbox lists it again
	rec = getReq(t, s, "/api/inbox")
	if rec.Code != http.StatusOK {
		t.Fatalf("inbox list code=%d body=%s", rec.Code, rec.Body.String())
	}
	var inbox []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &inbox); err != nil {
		t.Fatalf("decode inbox: %v", err)
	}
	if !containsID(inbox, "FEAT-950") {
		t.Fatalf("FEAT-950 missing from /api/inbox after recapture: %v", inbox)
	}

	// And the item row's status really is captured again.
	var status string
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT status FROM items WHERE repo_id=? AND item_id=?`,
		testRepoID, "FEAT-950",
	).Scan(&status); err != nil {
		t.Fatalf("status query: %v", err)
	}
	if status != "captured" {
		t.Fatalf("status=%q want captured after recapture", status)
	}
}

func getReq(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func containsID(rows []map[string]any, id string) bool {
	for _, r := range rows {
		if r["id"] == id {
			return true
		}
	}
	return false
}
