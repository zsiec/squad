package items

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/store"
)

var sectionHeader = regexp.MustCompile(`(?m)^## .+$`)

var (
	ErrCommentsRequired        = errors.New("refine: comments are required")
	ErrWrongStatusForRefine    = errors.New("refine: only captured or needs-refinement items can be refined")
	ErrWrongStatusForRecapture = errors.New("recapture: only needs-refinement items can be recaptured")
	ErrClaimNotHeld            = errors.New("recapture: claim not held by this agent")
)

func Refine(ctx context.Context, db *sql.DB, repoID, itemID, comments string) error {
	if strings.TrimSpace(comments) == "" {
		return ErrCommentsRequired
	}
	return store.WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		var path, status string
		err := tx.QueryRowContext(ctx,
			`SELECT path, status FROM items WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		).Scan(&path, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrItemNotFound
		}
		if err != nil {
			return err
		}
		if status != "captured" && status != "needs-refinement" {
			return fmt.Errorf("%w (status=%q)", ErrWrongStatusForRefine, status)
		}
		now := time.Now()
		if err := RewriteWithFeedback(path, comments, "needs-refinement", now); err != nil {
			return err
		}
		it, err := Parse(path)
		if err != nil {
			return err
		}
		return PersistOne(ctx, tx, repoID, it, false, now.Unix())
	})
}

func Recapture(ctx context.Context, db *sql.DB, repoID, itemID, agentID string) error {
	return store.WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		var path, status string
		err := tx.QueryRowContext(ctx,
			`SELECT path, status FROM items WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		).Scan(&path, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrItemNotFound
		}
		if err != nil {
			return err
		}
		if status != "needs-refinement" {
			return fmt.Errorf("%w (status=%q)", ErrWrongStatusForRecapture, status)
		}

		// Inline (rather than internal/claims.HolderOf) because claims
		// imports items — calling claims from here would close the cycle.
		var holder string
		err = tx.QueryRowContext(ctx,
			`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		).Scan(&holder)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && holder != agentID) {
			return ErrClaimNotHeld
		}
		if err != nil {
			return err
		}

		now := time.Now()
		date := now.UTC().Format("2006-01-02")
		if err := RewriteRecapture(path, date, "captured", now); err != nil {
			return err
		}
		it, err := Parse(path)
		if err != nil {
			return err
		}
		if err := PersistOne(ctx, tx, repoID, it, false, now.Unix()); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`DELETE FROM claims WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		)
		return err
	})
}

// RewriteWithFeedback reads the item file at path, transforms its body via
// WriteFeedback(comments), and updates frontmatter status to newStatus and
// updated to now. The whole rewrite is one atomicWrite so a crash mid-write
// cannot corrupt the file.
func RewriteWithFeedback(path, comments, newStatus string, now time.Time) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rewritten, err := rewriteFrontmatter(raw, map[string]string{
		"status":  newStatus,
		"updated": now.UTC().Format("2006-01-02"),
	})
	if err != nil {
		return fmt.Errorf("rewrite frontmatter for %s: %w", path, err)
	}
	fmEnd, body, err := splitRewrittenBody(rewritten)
	if err != nil {
		return err
	}
	newBody := WriteFeedback(body, comments)
	combined := append(append([]byte{}, rewritten[:fmEnd]...), []byte(newBody)...)
	return atomicWrite(path, combined)
}

// RewriteRecapture reads the item file at path, moves the working
// "## Reviewer feedback" section into "## Refinement history" (dated date),
// and updates frontmatter status to newStatus and updated to now. The whole
// rewrite is one atomicWrite so a crash mid-write cannot corrupt the file.
func RewriteRecapture(path, date, newStatus string, now time.Time) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rewritten, err := rewriteFrontmatter(raw, map[string]string{
		"status":  newStatus,
		"updated": now.UTC().Format("2006-01-02"),
	})
	if err != nil {
		return fmt.Errorf("rewrite frontmatter for %s: %w", path, err)
	}
	fmEnd, body, err := splitRewrittenBody(rewritten)
	if err != nil {
		return err
	}
	newBody := MoveFeedbackToHistory(body, date)
	combined := append(append([]byte{}, rewritten[:fmEnd]...), []byte(newBody)...)
	return atomicWrite(path, combined)
}

// splitRewrittenBody splits the output of rewriteFrontmatter (which always
// emits a normalized "---\n ... \n---\n<body>") into frontmatter bytes and
// the body string. rewriteFrontmatter normalizes BOM/CRLF, so the format is
// known.
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

func WriteFeedback(body, comments string) string {
	body = stripFeedback(body)
	feedback := "## Reviewer feedback\n" + strings.TrimRight(comments, "\n") + "\n\n"
	if idx := strings.Index(body, "## Problem"); idx >= 0 {
		return body[:idx] + feedback + body[idx:]
	}
	return feedback + body
}

func MoveFeedbackToHistory(body, date string) string {
	feedback, rest, ok := extractFeedback(body)
	if !ok {
		return body
	}
	round := nextRoundNumber(rest)
	entry := fmt.Sprintf("### Round %d — %s\n%s\n", round, date, strings.TrimRight(feedback, "\n"))

	if hi := strings.Index(rest, "## Refinement history"); hi >= 0 {
		histEnd := nextSection(rest, hi+len("## Refinement history"))
		return rest[:histEnd] + entry + "\n" + rest[histEnd:]
	}
	header := "## Refinement history\n" + entry + "\n"
	if pi := strings.Index(rest, "## Problem"); pi >= 0 {
		return rest[:pi] + header + rest[pi:]
	}
	return header + rest
}

func stripFeedback(body string) string {
	_, rest, ok := extractFeedback(body)
	if !ok {
		return body
	}
	return rest
}

func extractFeedback(body string) (feedback, rest string, ok bool) {
	hdr := "## Reviewer feedback\n"
	idx := strings.Index(body, hdr)
	if idx < 0 {
		return "", body, false
	}
	contentStart := idx + len(hdr)
	end := nextSection(body, contentStart)
	feedback = body[contentStart:end]
	rest = body[:idx] + body[end:]
	rest = strings.TrimLeft(rest, "\n")
	return feedback, rest, true
}

func nextSection(body string, start int) int {
	loc := sectionHeader.FindStringIndex(body[start:])
	if loc == nil {
		return len(body)
	}
	return start + loc[0]
}

func nextRoundNumber(body string) int {
	re := regexp.MustCompile(`(?m)^### Round (\d+)`)
	matches := re.FindAllStringSubmatch(body, -1)
	max := 0
	for _, m := range matches {
		var n int
		_, _ = fmt.Sscanf(m[1], "%d", &n)
		if n > max {
			max = n
		}
	}
	return max + 1
}
