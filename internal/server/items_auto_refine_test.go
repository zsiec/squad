package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/items"
)

const autoRefineCapturedItemFmt = `---
id: %s
title: a sufficiently long title for dor pass
type: feature
priority: P2
area: auth
status: %s
estimate: 1h
risk: low
created: 2026-04-25
updated: 2026-04-25
captured_by: agent-x
captured_at: 1700000000
---

## Acceptance criteria

- [ ] Specific, testable thing 1
- [ ] Specific, testable thing 2
`

const autoRefineCleanBody = "## Problem\n\nthe rule replaces the placeholder body verbatim.\n\n## Context\n\nthe rule replaces the placeholder body verbatim again.\n\n## Acceptance criteria\n- [ ] the rule replaces the placeholder body verbatim\n"

func newAutoRefineServer(t *testing.T) (*Server, string, string) {
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

func writeAutoRefineItem(t *testing.T, itemsDir, id, status string) string {
	t.Helper()
	path := filepath.Join(itemsDir, id+"-x.md")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(autoRefineCapturedItemFmt, id, status)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAutoRefineCommand_PassesPermissionFlagsAndDisablesHooks pins the exact
// argv and env of the spawned `claude -p` subprocess. Two non-obvious knobs:
//   - --allowedTools / --strict-mcp-config: claude -p has no interactive
//     permission prompt, so any unallowed tool call would die with "denied
//     by permissions on three attempts" and the handler would always 504.
//   - SQUAD_NO_HOOKS=1: the squad Stop hook runs `squad listen --max 24h`,
//     which holds the subprocess open after the response prints. Without
//     this, the spawned claude never exits and the handler always 504s.
func TestAutoRefineCommand_PassesPermissionFlagsAndDisablesHooks(t *testing.T) {
	cmd := autoRefineCommand(context.Background(), "the-prompt", "/tmp/cfg.json")

	wantArgs := []string{
		"claude", "-p", "the-prompt",
		"--mcp-config", "/tmp/cfg.json",
		"--strict-mcp-config",
		"--allowedTools", "mcp__squad__squad_get_item,mcp__squad__squad_inbox,mcp__squad__squad_history,mcp__squad__squad_auto_refine_apply",
	}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Errorf("argv mismatch:\n got: %v\nwant: %v", cmd.Args, wantArgs)
	}

	found := false
	for _, e := range cmd.Env {
		if e == "SQUAD_NO_HOOKS=1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("env missing SQUAD_NO_HOOKS=1; have: %v", cmd.Env)
	}
}

func TestAutoRefine_HappyPath(t *testing.T) {
	s, squadDir, itemsDir := newAutoRefineServer(t)
	path := writeAutoRefineItem(t, itemsDir, "BUG-700", "captured")

	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		if err := items.AutoRefineApply(squadDir, "BUG-700", autoRefineCleanBody, "claude"); err != nil {
			return autoRefineRunResult{Err: err}
		}
		return autoRefineRunResult{}
	})

	rec := postJSON(t, s, "/api/items/BUG-700/auto-refine", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["id"] != "BUG-700" {
		t.Errorf("response id=%v want BUG-700", got["id"])
	}
	if got["dor_pass"] != true {
		t.Errorf("response dor_pass=%v want true", got["dor_pass"])
	}
	if at, _ := got["auto_refined_at"].(float64); at == 0 {
		t.Errorf("response missing auto_refined_at: %v", got["auto_refined_at"])
	}
	if got["auto_refined_by"] != "claude" {
		t.Errorf("auto_refined_by=%v want claude", got["auto_refined_by"])
	}
	if body, _ := got["body_markdown"].(string); !strings.Contains(body, "the rule replaces the placeholder body verbatim") {
		t.Errorf("response body did not pick up new AC: %q", body)
	}

	on, err := items.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if on.Status != "captured" {
		t.Errorf("file status flipped to %q; auto-refine must keep captured", on.Status)
	}
}

func TestAutoRefine_ClaudeNotFoundReturns503(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-701", "captured")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		return autoRefineRunResult{Err: errClaudeNotFound}
	})
	rec := postJSON(t, s, "/api/items/BUG-701/auto-refine", map[string]any{})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "claude CLI not found on PATH") {
		t.Errorf("body should mention missing claude PATH, got %s", rec.Body.String())
	}
}

func TestAutoRefine_TimeoutReturns504(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-702", "captured")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		return autoRefineRunResult{Err: errors.New("deadline"), TimedOut: true}
	})
	rec := postJSON(t, s, "/api/items/BUG-702/auto-refine", map[string]any{})
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "timed out") {
		t.Errorf("body should mention timeout, got %s", rec.Body.String())
	}
}

func TestAutoRefine_NonZeroExitReturns502(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-703", "captured")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		return autoRefineRunResult{Err: errors.New("exit status 1"), Stderr: []byte("some failure detail")}
	})
	rec := postJSON(t, s, "/api/items/BUG-703/auto-refine", map[string]any{})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if !strings.Contains(body["stderr"].(string), "some failure detail") {
		t.Errorf("response should include stderr snippet, got %v", body)
	}
}

func TestAutoRefine_NonZeroExitTruncatesStderr(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-704", "captured")
	huge := strings.Repeat("x", 10000)
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		return autoRefineRunResult{Err: errors.New("exit status 1"), Stderr: []byte(huge)}
	})
	rec := postJSON(t, s, "/api/items/BUG-704/auto-refine", map[string]any{})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code=%d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if got := len(body["stderr"].(string)); got != 512 {
		t.Errorf("stderr length=%d want 512 (truncated)", got)
	}
}

func TestAutoRefine_NoWriteReturns500(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-705", "captured")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		return autoRefineRunResult{}
	})
	rec := postJSON(t, s, "/api/items/BUG-705/auto-refine", map[string]any{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "without drafting") {
		t.Errorf("body should mention 'without drafting', got %s", rec.Body.String())
	}
}

func TestAutoRefine_NonCapturedReturns409(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-706", "open")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		t.Fatalf("runner must not be called for non-captured items")
		return autoRefineRunResult{}
	})
	rec := postJSON(t, s, "/api/items/BUG-706/auto-refine", map[string]any{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "open") {
		t.Errorf("body should mention current status, got %s", rec.Body.String())
	}
}

func TestAutoRefine_UnknownItemReturns404(t *testing.T) {
	s, _, _ := newAutoRefineServer(t)
	rec := postJSON(t, s, "/api/items/BUG-999/auto-refine", map[string]any{})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAutoRefine_ConcurrentClickReturns409(t *testing.T) {
	s, squadDir, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-707", "captured")

	released := make(chan struct{})
	first := make(chan struct{})
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		close(first)
		<-released
		if err := items.AutoRefineApply(squadDir, "BUG-707", autoRefineCleanBody, "claude"); err != nil {
			return autoRefineRunResult{Err: err}
		}
		return autoRefineRunResult{}
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var firstRec *struct {
		code int
	}
	go func() {
		defer wg.Done()
		rec := postJSON(t, s, "/api/items/BUG-707/auto-refine", map[string]any{})
		firstRec = &struct{ code int }{rec.Code}
	}()
	<-first
	rec := postJSON(t, s, "/api/items/BUG-707/auto-refine", map[string]any{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second click code=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "already in flight") {
		t.Errorf("409 body should say 'already in flight', got %s", rec.Body.String())
	}
	close(released)
	wg.Wait()
	if firstRec == nil || firstRec.code != http.StatusOK {
		t.Errorf("first click should succeed; got %+v", firstRec)
	}
}

func TestAutoRefine_PublishesInboxChanged(t *testing.T) {
	s, squadDir, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-708", "captured")

	sub := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(sub)

	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		if err := items.AutoRefineApply(squadDir, "BUG-708", autoRefineCleanBody, "claude"); err != nil {
			return autoRefineRunResult{Err: err}
		}
		return autoRefineRunResult{}
	})

	rec := postJSON(t, s, "/api/items/BUG-708/auto-refine", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}

	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("did not see inbox_changed event with action=auto-refine")
		case ev := <-sub:
			if ev.Kind != "inbox_changed" {
				continue
			}
			if ev.Payload["item_id"] == "BUG-708" && ev.Payload["action"] == "auto-refine" {
				return
			}
		}
	}
}

func TestAutoRefine_SlotReleasedAfterFailure(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-709", "captured")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
		return autoRefineRunResult{Err: errClaudeNotFound}
	})
	for i := 0; i < 3; i++ {
		rec := postJSON(t, s, "/api/items/BUG-709/auto-refine", map[string]any{})
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("call %d code=%d want 503 body=%s", i, rec.Code, rec.Body.String())
		}
	}
}
