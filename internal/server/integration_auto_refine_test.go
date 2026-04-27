package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

// fmtIntegrationCapturedTemplate is the literal squad-new template body —
// kept inline here (not derived from items.stubTemplate) so this test
// breaks if the production template ever drifts away from what real
// `squad new` users hit. Title is short on purpose so the title-or-problem
// rule won't fire instead.
const intAutoRefineTemplateItem = `---
id: %s
title: investigate the flaky auth test we have
type: feature
priority: P2
area: auth
status: captured
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
captured_by: agent-x
captured_at: 1700000000
---

## Problem
What is wrong / what doesn't exist. 1–3 sentences.

## Context
Why this matters. Where in the codebase it lives. What's been tried.

## Acceptance criteria
- [ ] Specific, testable thing 1
- [ ] Specific, testable thing 2

## Notes
Optional design notes.
`

const intAutoRefineDraftedBody = "## Problem\n\nthe rule replaces the placeholder body verbatim.\n\n## Context\n\nthe rule replaces the placeholder body verbatim again.\n\n## Acceptance criteria\n- [ ] the rule replaces the placeholder body verbatim\n"

func newIntegrationAutoRefineServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	db := newTestDB(t)
	tmp := t.TempDir()
	itemsDir := filepath.Join(tmp, "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(db, testRepoID, Config{RepoID: testRepoID, SquadDir: tmp})
	t.Cleanup(s.Close)
	return s, tmp, itemsDir
}

func writeIntegrationItem(t *testing.T, itemsDir, id, body string) string {
	t.Helper()
	path := filepath.Join(itemsDir, id+"-x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertTemplatePlaceholderViolation(t *testing.T, path string) {
	t.Helper()
	it, err := items.Parse(path)
	if err != nil {
		t.Fatalf("pre-state parse: %v", err)
	}
	violations := items.DoRCheck(it)
	var found bool
	for _, v := range violations {
		if v.Rule == "template-not-placeholder" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("pre-state must trip template-not-placeholder; got %+v", violations)
	}
}

func TestIntegration_AutoRefine(t *testing.T) {
	t.Run("happy path full lifecycle", func(t *testing.T) {
		s, squadDir, itemsDir := newIntegrationAutoRefineServer(t)
		path := writeIntegrationItem(t, itemsDir, "FEAT-100",
			fmt.Sprintf(intAutoRefineTemplateItem, "FEAT-100"))
		assertTemplatePlaceholderViolation(t, path)

		var seenPrompt, seenConfigPath string
		s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
			seenPrompt = prompt
			seenConfigPath = mcpConfigPath
			if err := items.AutoRefineApply(squadDir, "FEAT-100", intAutoRefineDraftedBody, "claude"); err != nil {
				return autoRefineRunResult{Err: err}
			}
			return autoRefineRunResult{}
		})

		rec := postJSON(t, s, "/api/items/FEAT-100/auto-refine", map[string]any{})
		if rec.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}

		if !strings.Contains(seenPrompt, "FEAT-100") {
			t.Errorf("prompt missing item id: %q", seenPrompt)
		}
		if !strings.Contains(seenPrompt, "squad_auto_refine_apply") {
			t.Errorf("prompt should reference the write tool: %q", seenPrompt)
		}
		if seenConfigPath == "" {
			t.Errorf("mcp config path was not passed to the runner")
		}
		if !strings.HasSuffix(seenConfigPath, ".json") {
			t.Errorf("mcp config path %q should be a .json temp file", seenConfigPath)
		}
		if _, err := os.Stat(seenConfigPath); !os.IsNotExist(err) {
			t.Errorf("mcp config temp file %q should be cleaned up by the time runner returns; stat err=%v", seenConfigPath, err)
		}

		var resp items.Item
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.AutoRefinedAt == 0 {
			t.Errorf("response auto_refined_at should be > 0")
		}
		if !strings.Contains(resp.Body, "the rule replaces the placeholder body verbatim") {
			t.Errorf("response body did not pick up new AC: %q", resp.Body)
		}

		on, err := items.Parse(path)
		if err != nil {
			t.Fatalf("post-state parse: %v", err)
		}
		if on.AutoRefinedAt == 0 {
			t.Errorf("on-disk auto_refined_at not stamped")
		}
		if on.AutoRefinedBy != "claude" {
			t.Errorf("on-disk auto_refined_by=%q want claude", on.AutoRefinedBy)
		}
		if on.Status != "captured" {
			t.Errorf("status flipped to %q; auto-refine must keep captured", on.Status)
		}
		if violations := items.DoRCheck(on); len(violations) != 0 {
			t.Errorf("post-state still has DoR violations: %+v", violations)
		}
	})

	t.Run("subprocess clean exit but no write returns 500", func(t *testing.T) {
		s, _, itemsDir := newIntegrationAutoRefineServer(t)
		path := writeIntegrationItem(t, itemsDir, "FEAT-101",
			fmt.Sprintf(intAutoRefineTemplateItem, "FEAT-101"))
		before := mustReadIntegrationFile(t, path)

		s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
			return autoRefineRunResult{}
		})
		rec := postJSON(t, s, "/api/items/FEAT-101/auto-refine", map[string]any{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "without drafting") {
			t.Errorf("body should explain claude exited without drafting: %s", rec.Body.String())
		}

		after := mustReadIntegrationFile(t, path)
		if !bytes.Equal(before, after) {
			t.Errorf("file was mutated despite no write; before=%d after=%d bytes", len(before), len(after))
		}
	})

	t.Run("dedupe blocks concurrent click for same id", func(t *testing.T) {
		s, squadDir, itemsDir := newIntegrationAutoRefineServer(t)
		writeIntegrationItem(t, itemsDir, "FEAT-102",
			fmt.Sprintf(intAutoRefineTemplateItem, "FEAT-102"))

		first := make(chan struct{})
		release := make(chan struct{})
		s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
			close(first)
			<-release
			if err := items.AutoRefineApply(squadDir, "FEAT-102", intAutoRefineDraftedBody, "claude"); err != nil {
				return autoRefineRunResult{Err: err}
			}
			return autoRefineRunResult{}
		})

		var firstCode int
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := postJSON(t, s, "/api/items/FEAT-102/auto-refine", map[string]any{})
			firstCode = rec.Code
		}()
		<-first
		rec := postJSON(t, s, "/api/items/FEAT-102/auto-refine", map[string]any{})
		if rec.Code != http.StatusConflict {
			t.Fatalf("second click code=%d want 409 body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "already in flight") {
			t.Errorf("409 body should say 'already in flight': %s", rec.Body.String())
		}
		close(release)
		wg.Wait()
		if firstCode != http.StatusOK {
			t.Errorf("first click should complete 200 after release; got %d", firstCode)
		}
	})

	t.Run("non-captured status returns 409 with current status", func(t *testing.T) {
		s, _, itemsDir := newIntegrationAutoRefineServer(t)
		body := strings.Replace(
			fmt.Sprintf(intAutoRefineTemplateItem, "FEAT-103"),
			"status: captured", "status: open", 1)
		writeIntegrationItem(t, itemsDir, "FEAT-103", body)
		s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
			t.Fatalf("runner must not be invoked for non-captured items")
			return autoRefineRunResult{}
		})
		rec := postJSON(t, s, "/api/items/FEAT-103/auto-refine", map[string]any{})
		if rec.Code != http.StatusConflict {
			t.Fatalf("code=%d want 409 body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "open") {
			t.Errorf("409 body should include current status 'open': %s", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "captured") {
			t.Errorf("409 body should reference the captured-only contract: %s", rec.Body.String())
		}
	})
}

func mustReadIntegrationFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

