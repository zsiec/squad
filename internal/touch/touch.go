// Package touch tracks per-agent file editing declarations so peers can be
// warned when they reach for a file already in another agent's working set.
// State is persisted in the shared SQLite store.
package touch

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/store"
)

var ErrEmptyPath = errors.New("touch: path must not be empty")

// ErrPathTooLong reports a touch path beyond MaxPathLen bytes — POSIX
// PATH_MAX is 4096 on Linux, and anything longer almost certainly came
// from a hostile caller trying to inflate the touches table.
var ErrPathTooLong = errors.New("touch: path too long")

const MaxPathLen = 4096

// Tracker writes/reads against the touches table. Construction takes
// *sql.DB + repoID directly (no store wrapper exists in Phase 1).
type Tracker struct {
	db         *sql.DB
	repoID     string
	now        func() time.Time
	rootOnce   sync.Once
	cachedRoot string
}

func New(db *sql.DB, repoID string) *Tracker {
	return &Tracker{db: db, repoID: repoID, now: time.Now}
}

func NewWithClock(db *sql.DB, repoID string, clock func() time.Time) *Tracker {
	return &Tracker{db: db, repoID: repoID, now: clock}
}

func (t *Tracker) nowUnix() int64 { return t.now().Unix() }

// repoRoot returns the cached repo root, querying the repos table on
// first call. Returns "" when no repos row exists for repoID — callers
// fall back to leaving the path unchanged.
func (t *Tracker) repoRoot() string {
	t.rootOnce.Do(func() {
		_ = t.db.QueryRow(
			`SELECT COALESCE(root_path, '') FROM repos WHERE id = ?`, t.repoID,
		).Scan(&t.cachedRoot)
	})
	return t.cachedRoot
}

// normalizePath canonicalises every path the touches table sees. The
// pre-edit-touch hook records absolute paths (Claude Code emits
// tool_input.file_path absolute) while squad touch / squad claim
// --touches=... pass repo-relative paths; without this, the same
// logical file produces two distinct rows. Behaviour:
//
//   - empty path: unchanged.
//   - relative path: filepath.Clean.
//   - absolute path inside the repo root: filepath.Rel + Clean.
//   - absolute path outside the repo root (vendored deps, /usr/...,
//     symlinked tooling): kept absolute via Clean — the only sensible
//     identity when no repo-relative form exists.
//   - repo root unknown (no repos row): kept verbatim via Clean — better
//     than coercing to relative against "".
func (t *Tracker) normalizePath(p string) string {
	if p == "" {
		return p
	}
	if !filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	root := t.repoRoot()
	if root == "" {
		return filepath.Clean(p)
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.Clean(p)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Clean(p)
	}
	return filepath.Clean(rel)
}

// Add inserts a touch row for (agentID, path) within the tracker's repo.
// Returns the agent IDs that already hold this path open (excluding agentID).
func (t *Tracker) Add(ctx context.Context, agentID, itemID, path string) (conflicts []string, err error) {
	if path == "" {
		return nil, ErrEmptyPath
	}
	if len(path) > MaxPathLen {
		return nil, ErrPathTooLong
	}
	path = t.normalizePath(path)
	err = store.WithTxRetry(ctx, t.db, func(tx *sql.Tx) error {
		conflicts = conflicts[:0]
		rows, err := tx.QueryContext(ctx, `
			SELECT DISTINCT agent_id FROM touches
			WHERE path = ? AND repo_id = ? AND released_at IS NULL AND agent_id != ?
		`, path, t.repoID, agentID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				rows.Close()
				return err
			}
			conflicts = append(conflicts, a)
		}
		rows.Close()

		var item sql.NullString
		if itemID != "" {
			item = sql.NullString{String: itemID, Valid: true}
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
			VALUES (?, ?, ?, ?, ?)
		`, t.repoID, agentID, item, path, t.nowUnix())
		return err
	})
	if err != nil {
		return nil, err
	}
	return conflicts, nil
}

// EnsureActive is the idempotent variant of Add for high-frequency callers
// (the PreToolUse Edit/Write hook fires on every Edit, not just the first).
// It inserts a touch row only when the agent has no active touch on path
// already; either way it returns the conflict list a fresh Add would have
// surfaced. Repeated Edits to the same file by the same agent therefore
// produce a single row, while still keeping the conflict signal accurate.
func (t *Tracker) EnsureActive(ctx context.Context, agentID, itemID, path string) (conflicts []string, err error) {
	if path == "" {
		return nil, ErrEmptyPath
	}
	if len(path) > MaxPathLen {
		return nil, ErrPathTooLong
	}
	path = t.normalizePath(path)
	err = store.WithTxRetry(ctx, t.db, func(tx *sql.Tx) error {
		conflicts = conflicts[:0]
		rows, err := tx.QueryContext(ctx, `
			SELECT DISTINCT agent_id FROM touches
			WHERE path = ? AND repo_id = ? AND released_at IS NULL AND agent_id != ?
		`, path, t.repoID, agentID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				rows.Close()
				return err
			}
			conflicts = append(conflicts, a)
		}
		rows.Close()

		var has int
		err = tx.QueryRowContext(ctx, `
			SELECT 1 FROM touches
			WHERE repo_id = ? AND agent_id = ? AND path = ? AND released_at IS NULL
			LIMIT 1
		`, t.repoID, agentID, path).Scan(&has)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		var item sql.NullString
		if itemID != "" {
			item = sql.NullString{String: itemID, Valid: true}
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
			VALUES (?, ?, ?, ?, ?)
		`, t.repoID, agentID, item, path, t.nowUnix())
		return err
	})
	if err != nil {
		return nil, err
	}
	return conflicts, nil
}

func (t *Tracker) Release(ctx context.Context, agentID, path string) error {
	if path == "" {
		return ErrEmptyPath
	}
	path = t.normalizePath(path)
	_, err := t.db.ExecContext(ctx, `
		UPDATE touches SET released_at = ?
		WHERE repo_id = ? AND agent_id = ? AND path = ? AND released_at IS NULL
	`, t.nowUnix(), t.repoID, agentID, path)
	return err
}

func (t *Tracker) ReleaseAll(ctx context.Context, agentID string) (int, error) {
	res, err := t.db.ExecContext(ctx, `
		UPDATE touches SET released_at = ?
		WHERE repo_id = ? AND agent_id = ? AND released_at IS NULL
	`, t.nowUnix(), t.repoID, agentID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// ActiveTouch describes a peer's still-open touch — surfaced by ListOthers
// so hooks (and the dashboard) can show "agent X is editing path Y."
type ActiveTouch struct {
	AgentID   string `json:"agent_id"`
	Repo      string `json:"repo"`
	ItemID    string `json:"item_id"`
	Path      string `json:"path"`
	StartedAt int64  `json:"started_at,omitempty"`
}

// ListOthers returns every active touch in this repo NOT held by agentID.
// Hooks (pre-edit) call this to see whether an Edit would conflict with a peer.
// `Repo` is the project's friendly name when known (from repos.name), falling
// back to the basename of root_path, finally to repo_id.
func (t *Tracker) ListOthers(ctx context.Context, agentID string) ([]ActiveTouch, error) {
	repoLabel := t.repoID
	var name, root string
	_ = t.db.QueryRowContext(ctx,
		`SELECT COALESCE(name, ''), COALESCE(root_path, '') FROM repos WHERE id = ?`,
		t.repoID).Scan(&name, &root)
	switch {
	case name != "":
		repoLabel = name
	case root != "":
		// Use the directory basename (last path segment) without importing path.
		repoLabel = root
		for i := len(root) - 1; i >= 0; i-- {
			if root[i] == '/' || root[i] == '\\' {
				repoLabel = root[i+1:]
				break
			}
		}
	}

	rows, err := t.db.QueryContext(ctx, `
		SELECT agent_id, COALESCE(item_id, ''), path, started_at
		FROM touches
		WHERE repo_id = ? AND released_at IS NULL AND agent_id != ?
		ORDER BY started_at
	`, t.repoID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActiveTouch
	for rows.Next() {
		row := ActiveTouch{Repo: repoLabel}
		if err := rows.Scan(&row.AgentID, &row.ItemID, &row.Path, &row.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// ListOthersSince is ListOthers with a freshness floor: only peer touches
// whose started_at is at or after `since` are returned. The post-claim
// overlap nudge passes time.Now().Add(-24h) so day-old peer activity
// surfaces but week-old residue does not. Filtering happens in Go on top
// of ListOthers — fine for current scale (peer count × open files); push
// `started_at >= ?` into SQL when that no longer holds.
func (t *Tracker) ListOthersSince(ctx context.Context, agentID string, since time.Time) ([]ActiveTouch, error) {
	all, err := t.ListOthers(ctx, agentID)
	if err != nil {
		return nil, err
	}
	cutoff := since.Unix()
	out := all[:0]
	for _, r := range all {
		if r.StartedAt >= cutoff {
			out = append(out, r)
		}
	}
	return out, nil
}

// Conflicts returns the agent IDs (excluding agentID) currently holding path.
func (t *Tracker) Conflicts(ctx context.Context, agentID, path string) ([]string, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}
	path = t.normalizePath(path)
	rows, err := t.db.QueryContext(ctx, `
		SELECT DISTINCT agent_id FROM touches
		WHERE path = ? AND repo_id = ? AND released_at IS NULL AND agent_id != ?
	`, path, t.repoID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
