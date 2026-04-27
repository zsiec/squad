// Package items owns squad's on-disk item format (.squad/items/*.md with
// YAML frontmatter), the walk that materializes them into memory, and the
// rewrite helpers that mutate frontmatter atomically. Pure file-layer; the
// claim ledger and DB indexing live elsewhere.
package items

import (
	"regexp"
	"time"
)

// CountAC returns the number of acceptance-criteria checkbox lines (both
// `- [ ]` and `- [x]`, asterisk bullets allowed) under the `## Acceptance
// criteria` header. Returns 0 when the header is missing, when there are no
// checkboxes between the header and the next H2, or when body is empty.
func CountAC(body string) int {
	hdr := dorACHeaderRe.FindStringIndex(body)
	if hdr == nil {
		return 0
	}
	rest := body[hdr[1]:]
	if nxt := dorNextHdrRe.FindStringIndex(rest); nxt != nil {
		rest = rest[:nxt[0]]
	}
	return len(dorCheckboxRe.FindAllStringIndex(rest, -1))
}

var (
	// matches multi-segment slash paths like `cmd/squad/claim.go` or the
	// extension-less Go package path `internal/items/refine`. requires at
	// least one slash and word/dash characters either side, so e.g./i.e.
	// and bare prose dots stay clear.
	fileRefSlashedRe = regexp.MustCompile(`[a-zA-Z0-9_-]+(?:/[a-zA-Z0-9_.-]+)+`)
	// matches bare uppercase markdown docs like AGENTS.md / CLAUDE.md /
	// README.md so root-level project docs are catch-able even when not
	// path-prefixed.
	fileRefBareDocRe = regexp.MustCompile(`\b[A-Z][A-Z0-9_]+\.md\b`)
)

// CountFileRefs returns the number of distinct file/package references
// inside the AC bullets — used by the post-claim decompose nudge to spot
// items that span enough files to be worth splitting first. Inputs outside
// the AC section are ignored so prose paths in Problem/Context don't inflate
// the count.
func CountFileRefs(body string) int {
	hdr := dorACHeaderRe.FindStringIndex(body)
	if hdr == nil {
		return 0
	}
	rest := body[hdr[1]:]
	if nxt := dorNextHdrRe.FindStringIndex(rest); nxt != nil {
		rest = rest[:nxt[0]]
	}
	seen := map[string]struct{}{}
	for _, m := range fileRefSlashedRe.FindAllString(rest, -1) {
		seen[m] = struct{}{}
	}
	for _, m := range fileRefBareDocRe.FindAllString(rest, -1) {
		seen[m] = struct{}{}
	}
	return len(seen)
}

type CountReport struct {
	InProgress int
	Ready      int
	Blocked    int
	Done       int
}

func Counts(w WalkResult, now time.Time) CountReport {
	statusByID := map[string]string{}
	for _, it := range w.Active {
		statusByID[it.ID] = it.Status
	}
	for _, it := range w.Done {
		statusByID[it.ID] = "done"
	}
	var c CountReport
	for _, it := range w.Active {
		switch {
		case it.Status == "in_progress":
			c.InProgress++
		case it.Status == "blocked":
			c.Blocked++
		case gatedUntil(it, now) || hasOpenBlocker(it, statusByID):
			c.Blocked++
		default:
			c.Ready++
		}
	}
	c.Done = len(w.Done)
	return c
}
