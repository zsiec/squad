package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestRepoBadgeCssIsDistinctlyStyled pins the SPA contract that the
// repo badge has its own visual treatment, not the inherited fallback
// from the bare `.row-badge` rule. In workspace mode the SPA renders
// a `repo` chip on every item row when results span more than one
// repo; without a dedicated style it would look identical to the
// untyped fallback chip and be useless as a disambiguator.
//
// Reads the embedded style.css (the same bytes the daemon serves)
// and asserts:
//  1. a `.row-badge.repo` selector is declared,
//  2. that block contains both `color:` and `border-color:` declarations,
//  3. neither matches any of the existing `.row-badge.{epic,parallel,evidence}`
//     pairs (so the badge is visually distinct, not just a re-skin).
func TestRepoBadgeCssIsDistinctlyStyled(t *testing.T) {
	body, err := webFS.ReadFile("web/style.css")
	if err != nil {
		t.Fatalf("read embedded style.css: %v", err)
	}
	css := string(body)

	repoColor, repoBorder := extractRowBadgeColors(css, "repo")
	if repoColor == "" {
		t.Fatalf(".row-badge.repo missing or has no color: declaration\n\nexcerpt of file around row-badge:\n%s",
			rowBadgeExcerpt(css))
	}
	if repoBorder == "" {
		t.Fatalf(".row-badge.repo missing or has no border-color: declaration\n\nexcerpt of file around row-badge:\n%s",
			rowBadgeExcerpt(css))
	}

	for _, variant := range []string{"epic", "parallel", "evidence"} {
		c, b := extractRowBadgeColors(css, variant)
		if c == "" && b == "" {
			t.Fatalf("baseline variant .row-badge.%s missing — test fixture out of date", variant)
		}
		if c == repoColor {
			t.Errorf(".row-badge.repo color %q collides with .row-badge.%s color %q — pick a distinct hue",
				repoColor, variant, c)
		}
		if b == repoBorder {
			t.Errorf(".row-badge.repo border-color %q collides with .row-badge.%s border-color %q — pick a distinct shade",
				repoBorder, variant, b)
		}
	}
}

// extractRowBadgeColors finds the first `.row-badge.<variant>` block
// in css and returns the color and border-color values declared inside
// its braces. Returns "" for either when missing. Tolerates whitespace
// variation but not multi-line block notation.
func extractRowBadgeColors(css, variant string) (color, borderColor string) {
	pattern := regexp.MustCompile(`\.row-badge\.` + regexp.QuoteMeta(variant) + `\s*\{([^}]*)\}`)
	m := pattern.FindStringSubmatch(css)
	if m == nil {
		return "", ""
	}
	body := m[1]
	colorRe := regexp.MustCompile(`(?:^|;)\s*color\s*:\s*([^;]+)`)
	borderRe := regexp.MustCompile(`(?:^|;)\s*border-color\s*:\s*([^;]+)`)
	if c := colorRe.FindStringSubmatch(body); c != nil {
		color = strings.TrimSpace(c[1])
	}
	if b := borderRe.FindStringSubmatch(body); b != nil {
		borderColor = strings.TrimSpace(b[1])
	}
	return color, borderColor
}

func rowBadgeExcerpt(css string) string {
	idx := strings.Index(css, "/* board row badges")
	if idx < 0 {
		idx = strings.Index(css, ".row-badge")
	}
	if idx < 0 {
		return "(row-badge block not found)"
	}
	end := idx + 600
	if end > len(css) {
		end = len(css)
	}
	return css[idx:end]
}
