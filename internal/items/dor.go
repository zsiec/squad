package items

import (
	"regexp"
	"strings"
)

type DoRViolation struct {
	Rule    string `json:"rule"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

var (
	dorACHeaderRe      = regexp.MustCompile(`(?m)^##\s+Acceptance\s+criteria\s*$`)
	dorProblemRe       = regexp.MustCompile(`(?ms)^##\s+Problem\s*\n(.*?)(\n##\s+|\z)`)
	dorCheckboxRe      = regexp.MustCompile(`(?m)^\s*[-*]\s*\[[ xX]\]\s+`)
	dorCheckboxLabelRe = regexp.MustCompile(`(?m)^\s*[-*]\s*\[[ xX]\]\s+(.*)$`)
	dorNextHdrRe       = regexp.MustCompile(`(?m)^##\s+`)
)

func DoRCheck(it Item) []DoRViolation {
	var out []DoRViolation
	if it.Area == "" || it.Area == "<fill-in>" {
		out = append(out, DoRViolation{
			Rule: "area-set", Field: "area",
			Message: "area is unset or <fill-in>; set it to a real value",
		})
	}
	if !hasACCheckbox(it.Body) {
		out = append(out, DoRViolation{
			Rule: "acceptance-criterion", Field: "body",
			Message: "no acceptance criteria checkbox; add at least one '- [ ] ...' line under '## Acceptance criteria'",
		})
	} else if acIsTemplatePlaceholder(it.Body) {
		out = append(out, DoRViolation{
			Rule: "template-not-placeholder", Field: "body",
			Message: "acceptance criteria are still the squad-new template placeholders; replace them with real, testable conditions",
		})
	}
	if !titleOrProblemSubstantive(it) {
		out = append(out, DoRViolation{
			Rule: "title-or-problem", Field: "title|body",
			Message: "title is too short and Problem section is empty; either lengthen the title past 5 words or fill in '## Problem'",
		})
	}
	return out
}

func hasACCheckbox(body string) bool {
	rest, ok := acSection(body)
	if !ok {
		return false
	}
	return dorCheckboxRe.MatchString(rest)
}

func acIsTemplatePlaceholder(body string) bool {
	rest, ok := acSection(body)
	if !ok {
		return false
	}
	matches := dorCheckboxLabelRe.FindAllStringSubmatch(rest, -1)
	if len(matches) == 0 {
		return false
	}
	for _, m := range matches {
		if !isTemplatePlaceholder(m[1]) {
			return false
		}
	}
	return true
}

func isTemplatePlaceholder(label string) bool {
	label = strings.TrimSpace(label)
	for _, p := range TemplateACPlaceholders {
		if label == p {
			return true
		}
	}
	return false
}

func acSection(body string) (string, bool) {
	hdr := dorACHeaderRe.FindStringIndex(body)
	if hdr == nil {
		return "", false
	}
	rest := body[hdr[1]:]
	if nxt := dorNextHdrRe.FindStringIndex(rest); nxt != nil {
		rest = rest[:nxt[0]]
	}
	return rest, true
}

func titleOrProblemSubstantive(it Item) bool {
	if len(strings.Fields(it.Title)) > 5 {
		return true
	}
	m := dorProblemRe.FindStringSubmatch(it.Body)
	if m == nil {
		return false
	}
	return strings.TrimSpace(m[1]) != ""
}
