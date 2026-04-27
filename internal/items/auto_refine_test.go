package items

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const dorCleanBody = "## Problem\n\nthe drafted body has real prose here.\n\n## Context\n\nwith a real context section.\n\n## Acceptance criteria\n- [ ] the rule replaces the placeholder body verbatim\n"

func TestParse_RoundTripsAutoRefinedFields(t *testing.T) {
	dir := t.TempDir()
	itemsDir := filepath.Join(dir, "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: BUG-001\ntitle: a sufficiently long title for dor\ntype: bug\npriority: P2\narea: auth\nstatus: captured\nestimate: 1h\nrisk: low\nauto_refined_at: 1700000123\nauto_refined_by: claude\n---\n\n" + dorCleanBody
	path := filepath.Join(itemsDir, "BUG-001-x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	it, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if it.AutoRefinedAt != 1700000123 {
		t.Errorf("AutoRefinedAt=%d want 1700000123", it.AutoRefinedAt)
	}
	if it.AutoRefinedBy != "claude" {
		t.Errorf("AutoRefinedBy=%q want claude", it.AutoRefinedBy)
	}
}

func TestAutoRefineApply_HappyPath(t *testing.T) {
	dir, id := setupCapturedItem(t)
	if err := AutoRefineApply(dir, id, dorCleanBody, "", "claude"); err != nil {
		t.Fatalf("AutoRefineApply: %v", err)
	}
	path := mustItemPath(t, dir, id)
	it, err := Parse(path)
	if err != nil {
		t.Fatalf("parse after apply: %v", err)
	}
	if it.Status != "captured" {
		t.Errorf("status flipped to %q; auto-refine must keep captured→open human-only", it.Status)
	}
	if it.AutoRefinedAt == 0 {
		t.Errorf("AutoRefinedAt not stamped")
	}
	if it.AutoRefinedBy != "claude" {
		t.Errorf("AutoRefinedBy=%q want claude", it.AutoRefinedBy)
	}
	if !strings.Contains(it.Body, "the rule replaces the placeholder body verbatim") {
		t.Errorf("body did not pick up the new AC: %q", it.Body)
	}
	if violations := DoRCheck(it); len(violations) != 0 {
		t.Errorf("rewritten item still has DoR violations: %+v", violations)
	}
}

// TestAutoRefineApply_WritesAreaWhenSupplied covers the "claude auto-refine
// fills placeholder area" path. With a non-empty area arg, the frontmatter
// `area` field must be rewritten and the DoR area-set rule must pass even
// when the original captured item had area `<fill-in>`.
func TestAutoRefineApply_WritesAreaWhenSupplied(t *testing.T) {
	dir, id := setupCapturedItemWithArea(t, "<fill-in>")

	if err := AutoRefineApply(dir, id, dorCleanBody, "dashboard", "claude"); err != nil {
		t.Fatalf("AutoRefineApply: %v", err)
	}

	it, err := Parse(mustItemPath(t, dir, id))
	if err != nil {
		t.Fatal(err)
	}
	if it.Area != "dashboard" {
		t.Errorf("area=%q want dashboard", it.Area)
	}
	if violations := DoRCheck(it); len(violations) != 0 {
		t.Errorf("DoR violations after area-supplying apply: %+v", violations)
	}
}

// TestAutoRefineApply_PreservesAreaWhenEmpty covers the back-compat path:
// callers that don't supply an area must leave the frontmatter `area`
// untouched (the body-only refine that already worked before this change).
func TestAutoRefineApply_PreservesAreaWhenEmpty(t *testing.T) {
	dir, id := setupCapturedItemWithArea(t, "auth")

	if err := AutoRefineApply(dir, id, dorCleanBody, "", "claude"); err != nil {
		t.Fatalf("AutoRefineApply: %v", err)
	}

	it, err := Parse(mustItemPath(t, dir, id))
	if err != nil {
		t.Fatal(err)
	}
	if it.Area != "auth" {
		t.Errorf("area=%q want auth (must be untouched when not supplied)", it.Area)
	}
}

func TestAutoRefineApply_RefusesNonCapturedStatus(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := NewWithOptions(dir, "FEAT", "a sufficiently long title for dor", Options{
		Area:  "auth",
		Ready: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := mustParseID(t, path)
	beforeBytes := mustReadFile(t, path)

	err = AutoRefineApply(dir, id, dorCleanBody, "", "claude")
	if err == nil {
		t.Fatal("AutoRefineApply on open item must error")
	}
	if !strings.Contains(err.Error(), "captured") {
		t.Errorf("error should mention captured-only contract; got %v", err)
	}

	if afterBytes := mustReadFile(t, path); !equalBytes(beforeBytes, afterBytes) {
		t.Errorf("file mutated despite refusal")
	}
}

func TestAutoRefineApply_RefusesEmptyBody(t *testing.T) {
	dir, id := setupCapturedItem(t)
	path := mustItemPath(t, dir, id)
	beforeBytes := mustReadFile(t, path)

	for _, b := range []string{"", "   ", "\n\t\n"} {
		err := AutoRefineApply(dir, id, b, "", "claude")
		if err == nil {
			t.Errorf("AutoRefineApply with body %q must error", b)
		}
	}
	if afterBytes := mustReadFile(t, path); !equalBytes(beforeBytes, afterBytes) {
		t.Errorf("file mutated despite refusal")
	}
}

func TestAutoRefineApply_RefusesDoRFailingBody(t *testing.T) {
	dir, id := setupCapturedItem(t)
	path := mustItemPath(t, dir, id)
	beforeBytes := mustReadFile(t, path)

	cases := []struct {
		name     string
		body     string
		wantRule string
	}{
		{
			"placeholder template AC",
			"## Problem\n\nreal prose.\n\n## Acceptance criteria\n- [ ] " + TemplateACPlaceholders[0] + "\n- [ ] " + TemplateACPlaceholders[1] + "\n",
			"template-not-placeholder",
		},
		{
			"no AC checkbox",
			"## Problem\n\nprose without ac\n",
			"acceptance-criterion",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := AutoRefineApply(dir, id, c.body, "", "claude")
			if err == nil {
				t.Fatalf("AutoRefineApply must error on DoR-failing body")
			}
			if !strings.Contains(err.Error(), c.wantRule) {
				t.Errorf("error %q must name failing rule %q", err, c.wantRule)
			}
		})
	}

	if afterBytes := mustReadFile(t, path); !equalBytes(beforeBytes, afterBytes) {
		t.Errorf("file mutated despite refusal")
	}
}

func TestAutoRefineApply_RefusesUnknownItem(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := AutoRefineApply(dir, "BUG-999", dorCleanBody, "", "claude")
	if err == nil {
		t.Fatal("AutoRefineApply on unknown id must error")
	}
	if !strings.Contains(err.Error(), "BUG-999") {
		t.Errorf("error should name the missing id; got %v", err)
	}
}

func TestAutoRefineApply_RefusesItemAlreadyInDone(t *testing.T) {
	dir, id := setupCapturedItem(t)
	src := mustItemPath(t, dir, id)
	doneDir := filepath.Join(dir, "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, filepath.Join(doneDir, filepath.Base(src))); err != nil {
		t.Fatal(err)
	}
	err := AutoRefineApply(dir, id, dorCleanBody, "", "claude")
	if err == nil {
		t.Fatal("AutoRefineApply on done item must error")
	}
	if !strings.Contains(err.Error(), "done") {
		t.Errorf("error should mention done; got %v", err)
	}
}

func TestAutoRefineApply_ConcurrentCallsSerialize(t *testing.T) {
	dir, id := setupCapturedItem(t)

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			body := dorCleanBody + "\n<!-- caller=" + string(rune('A'+idx)) + " -->\n"
			errs <- AutoRefineApply(dir, id, body, "", "claude")
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent AutoRefineApply: %v", err)
		}
	}

	it, err := Parse(mustItemPath(t, dir, id))
	if err != nil {
		t.Fatalf("parse after concurrent applies: %v", err)
	}
	if it.AutoRefinedAt == 0 {
		t.Errorf("AutoRefinedAt not stamped after concurrent applies")
	}
	if violations := DoRCheck(it); len(violations) != 0 {
		t.Errorf("DoR violations after concurrent applies: %+v", violations)
	}
}

func setupCapturedItem(t *testing.T) (squadDir, id string) {
	t.Helper()
	return setupCapturedItemWithArea(t, "auth")
}

func setupCapturedItemWithArea(t *testing.T, area string) (squadDir, id string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := NewWithOptions(dir, "FEAT", "a sufficiently long title for dor", Options{
		Area: area,
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir, mustParseID(t, path)
}

func mustParseID(t *testing.T, path string) string {
	t.Helper()
	it, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func mustItemPath(t *testing.T, squadDir, id string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(squadDir, "items", id+"-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for %s, got %v", id, matches)
	}
	return matches[0]
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
