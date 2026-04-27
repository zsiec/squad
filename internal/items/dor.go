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
	out = append(out, vagueACBulletViolations(it)...)
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

// vagueACBulletAllowedVerbs is a closed allow-list seeded from FEAT-036's
// suggestion and tuned against squad's own items+done corpus to keep the
// false-positive rate near zero. Growing it further is fine when a real AC
// trips on a missing verb; resist the urge to grow it speculatively.
var vagueACBulletAllowedVerbs = map[string]struct{}{
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {}, "has": {}, "have": {}, "had": {},
	"should": {}, "must": {}, "can": {}, "will": {}, "may": {}, "shall": {}, "could": {}, "would": {},
	"do": {}, "does": {}, "did": {}, "exists": {}, "exist": {},
	"prints": {}, "reads": {}, "writes": {}, "returns": {}, "emits": {}, "sends": {}, "receives": {}, "logs": {}, "posts": {},
	"accepts": {}, "rejects": {}, "allows": {}, "denies": {}, "blocks": {}, "skips": {}, "fires": {}, "refuses": {},
	"creates": {}, "makes": {}, "builds": {}, "generates": {}, "renders": {}, "scaffolds": {}, "populates": {}, "produces": {}, "captures": {},
	"deletes": {}, "removes": {}, "drops": {}, "clears": {}, "purges": {},
	"updates": {}, "sets": {}, "replaces": {}, "mutates": {}, "rewrites": {}, "stamps": {}, "records": {}, "preserves": {}, "carries": {},
	"validates": {}, "parses": {}, "verifies": {}, "asserts": {}, "ensures": {}, "confirms": {}, "checks": {}, "detects": {}, "tracks": {},
	"fails": {}, "succeeds": {}, "errors": {}, "exits": {}, "passes": {}, "matches": {}, "lands": {}, "continues": {},
	"calls": {}, "invokes": {}, "registers": {}, "handles": {}, "dispatches": {}, "queues": {}, "exposes": {}, "schedules": {}, "routes": {},
	"covers": {}, "exercises": {}, "surfaces": {}, "reflects": {}, "raises": {}, "throws": {}, "mirrors": {}, "scopes": {},
	"transitions": {}, "flips": {}, "propagates": {}, "splits": {}, "wraps": {}, "encodes": {}, "decodes": {},
	"renames": {}, "moves": {}, "imports": {}, "loads": {}, "saves": {}, "stores": {}, "fetches": {}, "lists": {}, "queries": {}, "links": {}, "tags": {},
	"contains": {}, "includes": {}, "excludes": {}, "filters": {}, "sorts": {}, "groups": {}, "merges": {}, "supports": {},
	"shows": {}, "hides": {}, "displays": {}, "prompts": {}, "warns": {}, "informs": {}, "reports": {}, "names": {},
	"opens": {}, "closes": {}, "starts": {}, "stops": {}, "runs": {}, "uses": {}, "gets": {}, "gains": {}, "picks": {}, "persists": {},
}

var vagueACBulletStripPunctRe = regexp.MustCompile(`^[\W_]+|[\W_]+$`)

func vagueACBulletViolations(it Item) []DoRViolation {
	if it.Type != "feature" && it.Type != "bug" {
		return nil
	}
	rest, ok := acSection(it.Body)
	if !ok {
		return nil
	}
	matches := dorCheckboxLabelRe.FindAllStringSubmatch(rest, -1)
	if len(matches) == 0 {
		return nil
	}
	titleNorm := strings.ToLower(strings.TrimSpace(it.Title))
	var out []DoRViolation
	for _, m := range matches {
		label := strings.TrimSpace(m[1])
		if label == "" {
			continue
		}
		if isTemplatePlaceholder(label) {
			// already covered by template-not-placeholder; don't double-fire.
			continue
		}
		if msg := vagueACBulletReason(label, titleNorm); msg != "" {
			out = append(out, DoRViolation{
				Rule: "vague-acceptance-bullet", Field: "body",
				Message: msg,
			})
		}
	}
	return out
}

func vagueACBulletReason(label, titleNorm string) string {
	if strings.EqualFold(strings.TrimSpace(label), titleNorm) {
		return `bullet "` + label + `" restates the item title; expand it into a falsifiable condition`
	}
	tokens := strings.Fields(label)
	if len(tokens) < 6 {
		return `bullet "` + label + `" too short — at least 6 words required`
	}
	for _, raw := range tokens {
		tok := strings.ToLower(vagueACBulletStripPunctRe.ReplaceAllString(raw, ""))
		if _, ok := vagueACBulletAllowedVerbs[tok]; ok {
			return ""
		}
	}
	return `bullet "` + label + `" contains no recognized verb; rephrase as a proposition (e.g., "the X rejects/returns/validates Y")`
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
