package server

import (
	"strings"
	"testing"
)

// TestInboxAutoRefineToastConsumesBothStreams pins the SPA contract for
// the auto-refine toast renderer. The server already widens the 502
// (non-zero exit) and 500 (no-draft) responses to carry both `stdout`
// and `stderr` (see TestAutoRefine_NonZeroExitIncludesBothStreams /
// TestAutoRefine_NoWriteIncludesBothStreams), but the toast renderer in
// `inbox.js` only read `stderr` on 502 and neither field on 500 — so
// the new server data was reachable in DevTools but invisible in the
// dashboard toast that surfaced the original failure report.
//
// Same `webFS.ReadFile` structural-pin pattern as
// TestRepoBadgeCssIsDistinctlyStyled in repo_badge_css_test.go.
// Asserts that the auto-refine status switch in inbox.js references
// both `payload.stdout` and `payload.stderr`, so a future regression
// that drops one direction fails this test.
func TestInboxAutoRefineToastConsumesBothStreams(t *testing.T) {
	body, err := webFS.ReadFile("web/inbox.js")
	if err != nil {
		t.Fatalf("read embedded web/inbox.js: %v", err)
	}
	src := string(body)

	// Locate the auto-refine status switch. The function name is the
	// stable handle here; if it gets renamed the test should fail
	// loudly so the contract is re-pinned alongside.
	const handle = "autoRefineToastForStatus"
	start := strings.Index(src, handle)
	if start < 0 {
		t.Fatalf("inbox.js: %q not found; auto-refine toast handler renamed?", handle)
	}
	// Window over the function body — the handler is short, 600 bytes
	// is generous and stays well clear of unrelated callers.
	window := src[start:]
	if len(window) > 1200 {
		window = window[:1200]
	}

	if !strings.Contains(window, "stdout") {
		t.Errorf("inbox.js: %s must consume payload.stdout so claude diagnostics on 502 / 500 reach the toast; nearby source:\n%s",
			handle, window)
	}
	if !strings.Contains(window, "stderr") {
		t.Errorf("inbox.js: %s must consume payload.stderr so the alternate stream is also visible; nearby source:\n%s",
			handle, window)
	}
}
