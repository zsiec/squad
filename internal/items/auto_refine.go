package items

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// AutoRefineApply rewrites the body of an item in-place and stamps the
// auto-refine audit fields. The status field is preserved — re-refining
// an `open` item leaves it `open`, etc. The human Accept click is still
// the only path from captured to open; this function never promotes
// status.
//
// Allowed statuses: captured, open. Items in in_progress or already in
// done are rejected — concurrent body edits against a held claim cause
// data loss, and done items are immutable.
//
// area is optional. When non-empty the frontmatter `area` field is rewritten
// alongside the body — this lets the auto-refine flow heal items captured
// with the `<fill-in>` placeholder, which the DoR area-set rule would
// otherwise reject. An empty area leaves frontmatter `area` untouched, which
// is the back-compat path for callers that only refine the body.
func AutoRefineApply(squadDir, itemID, newBody, area, refinedBy string) error {
	if strings.TrimSpace(newBody) == "" {
		return errors.New("auto-refine: newBody is empty")
	}
	return withItemsLock(squadDir, func() error {
		path, inDone, err := FindByID(squadDir, itemID)
		if err != nil {
			return fmt.Errorf("auto-refine: %s: %w", itemID, err)
		}
		if inDone {
			return fmt.Errorf("auto-refine: item %s is already done", itemID)
		}
		it, err := Parse(path)
		if err != nil {
			return err
		}
		switch it.Status {
		case "captured", "open":
			// allowed
		default:
			return fmt.Errorf("auto-refine: status is %q (only captured or open items can be auto-refined)", it.Status)
		}

		candidate := it
		candidate.Body = newBody
		if area != "" {
			candidate.Area = area
		}
		if violations := DoRCheck(candidate); len(violations) > 0 {
			rules := make([]string, 0, len(violations))
			for _, v := range violations {
				rules = append(rules, v.Rule)
			}
			return fmt.Errorf("auto-refine: drafted body failed DoR (%s)", strings.Join(rules, ", "))
		}

		now := time.Now()
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updates := map[string]string{
			"auto_refined_at": strconv.FormatInt(now.Unix(), 10),
			"auto_refined_by": refinedBy,
			"updated":         now.UTC().Format("2006-01-02"),
		}
		if area != "" {
			updates["area"] = area
		}
		rewritten, err := rewriteFrontmatter(raw, updates)
		if err != nil {
			return fmt.Errorf("rewrite frontmatter for %s: %w", path, err)
		}
		fmEnd, _, err := splitRewrittenBody(rewritten)
		if err != nil {
			return err
		}
		body := newBody
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		combined := append(append([]byte{}, rewritten[:fmEnd]...), []byte(body)...)
		return atomicWrite(path, combined)
	})
}

func splitRewrittenBody(raw []byte) (fmEnd int, body string, err error) {
	open := []byte("---\n")
	closeM := []byte("\n---\n")
	if !bytes.HasPrefix(raw, open) {
		return 0, "", fmt.Errorf("rewritten file does not begin with frontmatter")
	}
	rest := raw[len(open):]
	idx := bytes.Index(rest, closeM)
	if idx < 0 {
		return 0, "", fmt.Errorf("rewritten file missing closing frontmatter marker")
	}
	end := len(open) + idx + len(closeM)
	return end, string(raw[end:]), nil
}
