package prmark

import "regexp"

// markerRE constrains the captured id to PREFIX-NUMBER form. Loosening to
// any non-whitespace string was tempting (and what shipped initially), but
// QA fuzzing showed it would happily extract `<script>` or HTML-encoded
// junk; downstream FindByID filtered, but defense-in-depth is cheap here.
var markerRE = regexp.MustCompile(`<!--\s*squad-item:\s*([A-Z][A-Z0-9]*-\d+)\s*-->`)

func Extract(body string) string {
	m := markerRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func Format(itemID string) string {
	return "<!-- squad-item: " + itemID + " -->"
}
