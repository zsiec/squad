package items

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/store"
)

var (
	ErrItemClaimed    = errors.New("reject: item is claimed; force-release first")
	ErrReasonRequired = errors.New("reject: reason is required")
)

type DoRError struct {
	Violations []DoRViolation
}

func (e *DoRError) Error() string {
	return fmt.Sprintf("definition of ready failed (%d violations)", len(e.Violations))
}

type rejectionLogEntry struct {
	Ts     int64  `json:"ts"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
	By     string `json:"by"`
}

func Reject(ctx context.Context, db *sql.DB, repoID, itemID, reason, by, squadDir string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	return store.WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		var path, title string
		err := tx.QueryRowContext(ctx,
			`SELECT path, title FROM items WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		).Scan(&path, &title)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var claimed int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM claims WHERE repo_id=? AND item_id=?`,
			repoID, itemID,
		).Scan(&claimed); err != nil {
			return err
		}
		if claimed > 0 {
			return ErrItemClaimed
		}
		if err := appendRejectionLog(squadDir, rejectionLogEntry{
			Ts: time.Now().Unix(), ID: itemID, Title: title, Reason: reason, By: by,
		}); err != nil {
			return fmt.Errorf("reject: log append failed: %w", err)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reject: remove file: %w", err)
		}
		_, err = tx.ExecContext(ctx,
			`DELETE FROM items WHERE repo_id=? AND item_id=?`, repoID, itemID)
		return err
	})
}

func appendRejectionLog(squadDir string, e rejectionLogEntry) error {
	if err := os.MkdirAll(squadDir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(squadDir, "rejected.log")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(body, '\n'))
	return err
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
