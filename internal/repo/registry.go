package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	// Match on base id.
	var existing string
	scanErr := tx.QueryRowContext(ctx, `SELECT root_path FROM repos WHERE id = ?`, base).Scan(&existing)
	if scanErr == nil {
		if existing == rootPath {
			if err := tx.Commit(); err != nil {
				return "", "", err
			}
			return base, "", nil
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
					return "", "", err
				}
				if err := tx.Commit(); err != nil {
					return "", "", err
				}
				return candidate,
					fmt.Sprintf("squad: second clone of %s detected at %s — assigned id %s (path discriminator)", remoteURL, rootPath, candidate),
					nil
			}
			if err != nil {
				return "", "", err
			}
			if path == rootPath {
				if err := tx.Commit(); err != nil {
					return "", "", err
				}
				return candidate, "", nil
			}
		}
		return "", "", fmt.Errorf("repo: too many clones of %s (>99)", remoteURL)
	}
	if !errors.Is(scanErr, sql.ErrNoRows) {
		return "", "", scanErr
	}

	// Fresh insert at base id.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO repos (id, root_path, remote_url, name, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, base, rootPath, remoteURL, "", r.now().Unix()); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return base, "", nil
}
