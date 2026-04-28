package server

import (
	"strings"
	"testing"
)

// TestInboxRefineComposerUsesAutoRefineContract pins the SPA contract
// for the comment-driven refinement composer in inbox.js. The composer
// targets the auto-refine endpoint with payload
// `{comments: [{quoted_span, comment}, ...]}` (per the FEAT-062 server
// contract); the legacy peer-queue refine POST is no longer reachable
// from the inbox details flow.
//
// Same `webFS.ReadFile` structural-pin pattern as
// TestRepoBadgeCssIsDistinctlyStyled in repo_badge_css_test.go and
// TestInboxAutoRefineToastConsumesBothStreams in
// inbox_auto_refine_toast_test.go. A future refactor that drops the
// `quoted_span` field name, drops the `comments` array shape, or
// re-introduces a POST to `/api/items/{id}/refine` will fail this test.
func TestInboxRefineComposerUsesAutoRefineContract(t *testing.T) {
	body, err := webFS.ReadFile("web/inbox.js")
	if err != nil {
		t.Fatalf("read embedded web/inbox.js: %v", err)
	}
	src := string(body)

	if !strings.Contains(src, "quoted_span") {
		t.Errorf("inbox.js must reference the `quoted_span` field name " +
			"so each commented span is sent in the FEAT-062-shaped payload")
	}
	if !strings.Contains(src, "comments") {
		t.Errorf("inbox.js must reference the `comments` array " +
			"so the auto-refine endpoint receives the comment list")
	}
	if !strings.Contains(src, "/auto-refine") {
		t.Errorf("inbox.js must POST to /api/items/{id}/auto-refine")
	}

	// The legacy peer-queue refine POST has the form
	// `/api/items/${encodeURIComponent(id)}/refine` — i.e. it ends with
	// `id)}/refine` and crucially is NOT followed by `auto-`. The
	// auto-refine URL has `id)}/auto-refine` so it does not collide.
	if strings.Contains(src, "id)}/refine") {
		t.Errorf("inbox.js must not POST to the legacy /api/items/{id}/refine endpoint; " +
			"the comment-driven flow targets /auto-refine")
	}
}
