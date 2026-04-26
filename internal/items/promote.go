package items

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/zsiec/squad/internal/store"
)

type DoRError struct {
	Violations []DoRViolation
}

func (e *DoRError) Error() string {
	return fmt.Sprintf("definition of ready failed (%d violations)", len(e.Violations))
}

func Promote(ctx context.Context, db *sql.DB, repoID, itemID, acceptedBy string) error {
	return store.WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		var path, status string
		err := tx.QueryRowContext(ctx,
			`SELECT path, status FROM items WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		).Scan(&path, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("promote %s: item not found", itemID)
		}
		if err != nil {
			return err
		}
		if status == "open" {
			return nil
		}
		if status != "captured" {
			return fmt.Errorf("promote %s: status is %q (only captured items can be promoted)", itemID, status)
		}
		it, err := Parse(path)
		if err != nil {
			return err
		}
		if violations := DoRCheck(it); len(violations) > 0 {
			return &DoRError{Violations: violations}
		}
		nowUnix := time.Now().Unix()
		if err := rewritePromote(path, acceptedBy, nowUnix); err != nil {
			return err
		}
		it.Status = "open"
		it.AcceptedBy = acceptedBy
		it.AcceptedAt = nowUnix
		return PersistOne(ctx, tx, repoID, it, false, nowUnix)
	})
}

func rewritePromote(path, acceptedBy string, acceptedAt int64) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rewritten, err := rewriteFrontmatter(raw, map[string]string{
		"status":      "open",
		"accepted_by": acceptedBy,
		"accepted_at": strconv.FormatInt(acceptedAt, 10),
		"updated":     time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		return fmt.Errorf("rewrite frontmatter for %s: %w", path, err)
	}
	return atomicWrite(path, rewritten)
}
