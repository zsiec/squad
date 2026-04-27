package items

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Status stays `captured` on success — the human Accept click remains the
// only path from captured to open.
func AutoRefineApply(squadDir, itemID, newBody, refinedBy string) error {
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
		if it.Status != "captured" {
			return fmt.Errorf("auto-refine: status is %q (only captured items can be auto-refined)", it.Status)
		}

		candidate := it
		candidate.Body = newBody
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
		rewritten, err := rewriteFrontmatter(raw, map[string]string{
			"auto_refined_at": strconv.FormatInt(now.Unix(), 10),
			"auto_refined_by": refinedBy,
			"updated":         now.UTC().Format("2006-01-02"),
		})
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
