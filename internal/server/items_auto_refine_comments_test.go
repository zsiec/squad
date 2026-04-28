package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
)

// TestAutoRefinePromptFor_EmptyByteIdentical pins that an empty
// comment slice yields a prompt byte-identical to the legacy single-
// arg shape. Today's auto-refine button posts an empty body; without
// this guarantee, that path silently changes prompt content and the
// runner sees a different instruction.
func TestAutoRefinePromptFor_EmptyByteIdentical(t *testing.T) {
	got := autoRefinePromptFor("BUG-100", nil)
	want := autoRefineLegacyPromptFor("BUG-100")
	if got != want {
		t.Errorf("empty-comments prompt diverged from legacy text\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	got2 := autoRefinePromptFor("BUG-100", []AutoRefineComment{})
	if got2 != want {
		t.Errorf("zero-length slice should match nil; got divergence")
	}
}

// TestAutoRefinePromptFor_RendersComments pins the comment block
// shape: each {quoted_span, comment} pair renders as
//
//	> <span>
//	<comment>
//
// blocks above the existing instruction text, in input order.
func TestAutoRefinePromptFor_RendersComments(t *testing.T) {
	comments := []AutoRefineComment{
		{QuotedSpan: "the dashboard shows the wrong repo", Comment: "rephrase as a falsifiable AC bullet"},
		{QuotedSpan: "auth flow is broken", Comment: "name which auth provider — OIDC? SAML?"},
	}
	got := autoRefinePromptFor("BUG-101", comments)

	for _, c := range comments {
		if !strings.Contains(got, "> "+c.QuotedSpan+"\n"+c.Comment) {
			t.Errorf("prompt missing comment block for %q\nfull prompt:\n%s", c.QuotedSpan, got)
		}
	}
	// Order: first comment must appear before second.
	idx0 := strings.Index(got, comments[0].QuotedSpan)
	idx1 := strings.Index(got, comments[1].QuotedSpan)
	if idx0 < 0 || idx1 < 0 {
		t.Fatalf("at least one quoted_span missing")
	}
	if idx0 > idx1 {
		t.Errorf("comment order inverted: idx[0]=%d idx[1]=%d", idx0, idx1)
	}
	// Comments precede the legacy instruction text.
	legacyIdx := strings.Index(got, "You are auto-refining")
	if legacyIdx < 0 {
		t.Fatalf("legacy instruction header missing: %s", got)
	}
	if idx0 >= legacyIdx {
		t.Errorf("comment blocks must appear before the legacy instruction header")
	}
}

// TestAutoRefine_AcceptsAllNonInProgressStatuses pins the broadened
// precondition: captured and open both reach the runner; only
// in_progress is rejected. The previous gate (captured-only) blocked
// the use case described in the parent item: re-refining an
// already-auto-refined draft (status: open) with operator comments.
func TestAutoRefine_AcceptsAllNonInProgressStatuses(t *testing.T) {
	cases := []struct {
		status string
		id     string
	}{
		{"captured", "BUG-720"},
		{"open", "BUG-722"},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			s, squadDir, itemsDir := newAutoRefineServer(t)
			writeAutoRefineItem(t, itemsDir, c.id, c.status)
			s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath, repoRoot string) autoRefineRunResult {
				if err := items.AutoRefineApply(squadDir, c.id, autoRefineCleanBody, "", "claude"); err != nil {
					return autoRefineRunResult{Err: err}
				}
				return autoRefineRunResult{}
			})
			rec := postJSON(t, s, "/api/items/"+c.id+"/auto-refine", map[string]any{})
			if rec.Code != http.StatusOK {
				t.Errorf("status=%s: code=%d body=%s — broadened gate must allow %s",
					c.status, rec.Code, rec.Body.String(), c.status)
			}
		})
	}
}

// TestAutoRefine_InProgressReturns409 pins the only remaining
// rejected state. An item held by an agent must not have its body
// rewritten under it; concurrent body edits on a held claim cause
// data loss.
func TestAutoRefine_InProgressReturns409(t *testing.T) {
	s, _, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-723", "in_progress")
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath, repoRoot string) autoRefineRunResult {
		t.Fatalf("runner must not be called for in_progress items")
		return autoRefineRunResult{}
	})
	rec := postJSON(t, s, "/api/items/BUG-723/auto-refine", map[string]any{})
	if rec.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s want 409", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "in_progress") {
		t.Errorf("409 body should name the blocking status; got %s", rec.Body.String())
	}
}

// TestAutoRefine_CommentsReachClaudePrompt pins the request-body
// → prompt-string wire: the SPA posts `{comments: [{quoted_span,
// comment}]}` and the spawned claude must see those strings in its
// prompt. Without this, the SPA's range-selection UI is decorative.
func TestAutoRefine_CommentsReachClaudePrompt(t *testing.T) {
	s, squadDir, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-724", "captured")

	var seen string
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath, repoRoot string) autoRefineRunResult {
		seen = prompt
		if err := items.AutoRefineApply(squadDir, "BUG-724", autoRefineCleanBody, "", "claude"); err != nil {
			return autoRefineRunResult{Err: err}
		}
		return autoRefineRunResult{}
	})
	body := map[string]any{
		"comments": []map[string]string{
			{"quoted_span": "first selected paragraph", "comment": "tighten this AC"},
			{"quoted_span": "second selected sentence", "comment": "wrong area — should be auth not intake"},
		},
	}
	rec := postJSON(t, s, "/api/items/BUG-724/auto-refine", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		"first selected paragraph",
		"tighten this AC",
		"second selected sentence",
		"wrong area",
	} {
		if !strings.Contains(seen, want) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", want, seen)
		}
	}
}

// TestAutoRefine_DoRRejectionStaysIntact pins that a runner returning
// a placeholder body (e.g. "Specific, testable thing 1") still gets
// rejected by AutoRefineApply's DoR check, and the item's body stays
// untouched. The broadened status gate must not weaken the validation
// the apply path already enforces.
func TestAutoRefine_DoRRejectionStaysIntact(t *testing.T) {
	s, squadDir, itemsDir := newAutoRefineServer(t)
	writeAutoRefineItem(t, itemsDir, "BUG-725", "captured")

	placeholder := "## Problem\n\nstub.\n\n## Acceptance criteria\n- [ ] Specific, testable thing 1\n"
	s.SetAutoRefineRunner(func(ctx context.Context, prompt, mcpConfigPath, repoRoot string) autoRefineRunResult {
		if err := items.AutoRefineApply(squadDir, "BUG-725", placeholder, "", "claude"); err == nil {
			t.Errorf("AutoRefineApply must reject placeholder body; got nil err")
		}
		return autoRefineRunResult{}
	})
	rec := postJSON(t, s, "/api/items/BUG-725/auto-refine", map[string]any{})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("placeholder body should produce 500 (no-draft path); got %d body=%s",
			rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "without drafting") {
		t.Errorf("500 body should mention 'without drafting' since AutoRefineApply rejected the placeholder; got %s",
			rec.Body.String())
	}
}
