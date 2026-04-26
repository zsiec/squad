package items

// All single-item writes to .md files MUST call Persist (or PersistOne inside
// an existing tx) immediately after the disk write. Mirror is the bulk
// reconciler for periodic full walks; relying on Mirror to catch up after a
// hot-path mutation creates visible lag in the dashboard and `workspace
// status`.

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/zsiec/squad/internal/store"
)

const persistUpsert = `
INSERT INTO items (repo_id, item_id, title, type, priority, area, status, estimate,
                   risk, not_before, ac_total, ac_checked, archived, path, updated_at,
                   epic_id, parallel, conflicts_with)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(repo_id, item_id) DO UPDATE SET
  title=excluded.title, type=excluded.type, priority=excluded.priority,
  area=excluded.area, status=excluded.status, estimate=excluded.estimate,
  risk=excluded.risk, not_before=excluded.not_before, ac_total=excluded.ac_total,
  ac_checked=excluded.ac_checked, archived=excluded.archived, path=excluded.path,
  updated_at=excluded.updated_at, epic_id=excluded.epic_id,
  parallel=excluded.parallel, conflicts_with=excluded.conflicts_with
`

func Persist(ctx context.Context, db *sql.DB, repoID string, it Item, archived bool) error {
	return store.WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		return PersistOne(ctx, tx, repoID, it, archived, time.Now().Unix())
	})
}

func PersistOne(ctx context.Context, tx *sql.Tx, repoID string, it Item, archived bool, ts int64) error {
	status := it.Status
	if archived {
		status = "done"
	}
	parallel := 0
	if it.Parallel {
		parallel = 1
	}
	confJSON, err := json.Marshal(it.ConflictsWith)
	if err != nil {
		return err
	}
	// json.Marshal returns "null" for a nil slice; coerce to "[]" so json_each works on the column.
	if len(it.ConflictsWith) == 0 {
		confJSON = []byte("[]")
	}
	var epic sql.NullString
	if it.Epic != "" {
		epic = sql.NullString{String: it.Epic, Valid: true}
	}
	archivedFlag := 0
	if archived {
		archivedFlag = 1
	}
	_, err = tx.ExecContext(ctx, persistUpsert,
		repoID, it.ID, it.Title, it.Type, it.Priority, it.Area, status,
		it.Estimate, it.Risk, it.NotBefore, it.ACTotal, it.ACChecked,
		archivedFlag, it.Path, ts,
		epic, parallel, string(confJSON),
	)
	return err
}
