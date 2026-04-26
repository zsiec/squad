// Package touch tracks per-agent file editing declarations so peers can be
// warned when they reach for a file already in another agent's working set.
// State is persisted in the shared SQLite store.
package touch

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrEmptyPath = errors.New("touch: path must not be empty")

// MaxPathLen caps the bytes squad stores for a touched path. POSIX PATH_MAX
// is 4096 on Linux; anything longer almost certainly came from a hostile
// caller trying to inflate the touches table.
var ErrPathTooLong = errors.New("touch: path too long")

const MaxPathLen = 4096

// Tracker writes/reads against the touches table. Construction takes
// *sql.DB + repoID directly (no store wrapper exists in Phase 1).
type Tracker struct {
	db     *sql.DB
	repoID string
	now    func() time.Time
}

func New(db *sql.DB, repoID string) *Tracker {
	return &Tracker{db: db, repoID: repoID, now: time.Now}
}

func NewWithClock(db *sql.DB, repoID string, clock func() time.Time) *Tracker {
	return &Tracker{db: db, repoID: repoID, now: clock}
}

func (t *Tracker) nowUnix() int64 { return t.now().Unix() }

// Add inserts a touch row for (agentID, path) within the tracker's repo.
// Returns the agent IDs that already hold this path open (excluding agentID).
func (t *Tracker) Add(ctx context.Context, agentID, itemID, path string) (conflicts []string, err error) {
	if path == "" {
		return nil, ErrEmptyPath
	}
	if len(path) > MaxPathLen {
		return nil, ErrPathTooLong
	}
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT agent_id FROM touches
		WHERE path = ? AND repo_id = ? AND released_at IS NULL AND agent_id != ?
	`, path, t.repoID, agentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			rows.Close()
			return nil, err
		}
		conflicts = append(conflicts, a)
	}
	rows.Close()

	var item sql.NullString
	if itemID != "" {
		item = sql.NullString{String: itemID, Valid: true}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
		VALUES (?, ?, ?, ?, ?)
	`, t.repoID, agentID, item, path, t.nowUnix()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return conflicts, nil
}

func (t *Tracker) Release(ctx context.Context, agentID, path string) error {
	if path == "" {
		return ErrEmptyPath
	}
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
	AgentID string `json:"agent_id"`
	Repo    string `json:"repo"`
	ItemID  string `json:"item_id"`
	Path    string `json:"path"`
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
		SELECT agent_id, COALESCE(item_id, ''), path
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
		if err := rows.Scan(&row.AgentID, &row.ItemID, &row.Path); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// Conflicts returns the agent IDs (excluding agentID) currently holding path.
func (t *Tracker) Conflicts(ctx context.Context, agentID, path string) ([]string, error) {
	if path == "" {
		return nil, ErrEmptyPath
	}
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
