// Package repo discovers the working repo's identity (git-remote-derived id
// with a path-based fallback), maintains the per-machine repos table, and
// resolves canonical paths for cross-repo workspace queries.
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zsiec/squad/internal/store"
)

// RepoID is the canonical hash-based ID for a (remoteURL, rootPath) pair.
// Same wire format as DeriveRepoID — kept as a separate exported name so
// callers in the workspace package can use the cleaner spelling.
func RepoID(remoteURL, rootPath string) string {
	return DeriveRepoID(remoteURL, rootPath)
}

// Registry is a higher-level wrapper over the repos table that handles
// "second clone of the same remote" by appending a _2/_3/... suffix.
type Registry struct {
	db  *sql.DB
	now func() time.Time
}

func NewRegistry(db *sql.DB) *Registry {
	return &Registry{db: db, now: time.Now}
}

// Register upserts (or suffixes) a repo row. Returns the assigned id, an
// optional human-facing warning (empty when the row was a clean match), and
// any error.
//
//   - Same remote + same root_path: bump created_at-as-activity, return base id.
//   - Same remote + different root_path: scan _2..._99, insert at first free slot.
//   - No row at all: insert at base id.
func (r *Registry) Register(remoteURL, rootPath string) (id, warning string, err error) {
	base := DeriveRepoID(remoteURL, rootPath)
	ctx := context.Background()

	var resID, resWarn string
	err = store.WithTxRetry(ctx, r.db, func(tx *sql.Tx) error {
		resID, resWarn = "", ""
		var existing string
		scanErr := tx.QueryRowContext(ctx, `SELECT root_path FROM repos WHERE id = ?`, base).Scan(&existing)
		if scanErr == nil {
			if existing == rootPath {
				resID = base
				return nil
			}
			// Different root for same id — scan for _N suffix slot.
			for n := 2; n <= 99; n++ {
				candidate := fmt.Sprintf("%s_%d", base, n)
				var path string
				err := tx.QueryRowContext(ctx, `SELECT root_path FROM repos WHERE id = ?`, candidate).Scan(&path)
				if errors.Is(err, sql.ErrNoRows) {
					if _, err := tx.ExecContext(ctx, `
						INSERT INTO repos (id, root_path, remote_url, name, created_at)
						VALUES (?, ?, ?, ?, ?)
					`, candidate, rootPath, remoteURL, "", r.now().Unix()); err != nil {
						return err
					}
					resID = candidate
					resWarn = fmt.Sprintf("squad: second clone of %s detected at %s — assigned id %s (path discriminator)", remoteURL, rootPath, candidate)
					return nil
				}
				if err != nil {
					return err
				}
				if path == rootPath {
					resID = candidate
					return nil
				}
			}
			return fmt.Errorf("repo: too many clones of %s (>99)", remoteURL)
		}
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return scanErr
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO repos (id, root_path, remote_url, name, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, base, rootPath, remoteURL, "", r.now().Unix()); err != nil {
			return err
		}
		resID = base
		return nil
	})
	if err != nil {
		return "", "", err
	}
	return resID, resWarn, nil
}
