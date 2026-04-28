package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestInsightsPanelHorizontalScroll pins the contract that the stats
// panel can horizontal-scroll its grid when the panel width is too
// narrow for the natural 2-column tile layout. Without an explicit
// overflow rule, content is clipped by the panel and tiles become
// unreachable on narrow viewports. The fix lives entirely in
// internal/server/web/style.css — `.insights-body` must declare
// `overflow-x` (so the body scrolls), and `.insights .insights-grid`
// must declare a `min-width` (so the grid actually exceeds the body
// width when the body is narrow, rather than collapsing).
func TestInsightsPanelHorizontalScroll(t *testing.T) {
	body, err := webFS.ReadFile("web/style.css")
	if err != nil {
		t.Fatalf("read embedded style.css: %v", err)
	}
	css := string(body)

	bodyBlock := extractRuleBlock(css, `\.insights-body`)
	if bodyBlock == "" {
		t.Fatal(".insights-body rule not found — test fixture out of date")
	}
	if !regexp.MustCompile(`(?:^|;|\s)\s*overflow-x\s*:\s*(auto|scroll)`).MatchString(bodyBlock) {
		t.Errorf(".insights-body missing 'overflow-x: auto|scroll' — content will clip on narrow panel widths\n  rule: %s", bodyBlock)
	}

	gridBlock := extractRuleBlock(css, `\.insights\s+\.insights-grid`)
	if gridBlock == "" {
		t.Fatal(".insights .insights-grid rule not found — test fixture out of date")
	}
	if !regexp.MustCompile(`(?:^|;|\s)\s*min-width\s*:\s*\d+`).MatchString(gridBlock) {
		t.Errorf(".insights .insights-grid missing 'min-width: <px>' — without a floor the grid shrinks to fit the body and never triggers horizontal scroll\n  rule: %s", gridBlock)
	}

	// The mobile 1-col fallback must override min-width back to auto, or
	// it inherits the desktop floor and forces horizontal scroll on
	// viewports where the single-column layout was meant to fit.
	mobileBlock := regexp.MustCompile(`@media\s*\(\s*max-width:\s*720px\s*\)\s*\{[^}]*\.insights-grid\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if mobileBlock == nil {
		t.Fatal("@media (max-width: 720px) .insights-grid override missing — test fixture out of date")
	}
	if !regexp.MustCompile(`min-width\s*:\s*(auto|0)`).MatchString(mobileBlock[1]) {
		t.Errorf("mobile fallback must reset min-width:auto so 1-col layout doesn't inherit the desktop floor\n  rule: %s", mobileBlock[1])
	}
}

// extractRuleBlock returns the body (between braces) of the first CSS
// rule whose selector matches selectorRe (a regex fragment, no anchors).
// Returns "" when not found. Does not handle nested braces — fine for
// flat top-level rules in style.css.
func extractRuleBlock(css, selectorRe string) string {
	pattern := regexp.MustCompile(selectorRe + `\s*\{([^}]*)\}`)
	m := pattern.FindStringSubmatch(css)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}
