package items

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func Mirror(ctx context.Context, db *sql.DB, repoID string, w WalkResult) error {
	tx, err := store.BeginImmediate(ctx, db)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const upsert = `
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
	now := time.Now().Unix()
	keep := make(map[string]struct{}, len(w.Active)+len(w.Done))
	for _, group := range []struct {
		items    []Item
		archived int
	}{
		{w.Active, 0},
		{w.Done, 1},
	} {
		for _, it := range group.items {
			status := it.Status
			if group.archived == 1 {
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
			// json.Marshal returns "null" for a nil slice; coerce to "[]" so the column shape stays uniform for json_each.
			if len(it.ConflictsWith) == 0 {
				confJSON = []byte("[]")
			}
			var epic sql.NullString
			if it.Epic != "" {
				epic = sql.NullString{String: it.Epic, Valid: true}
			}
			if _, err := tx.ExecContext(ctx, upsert,
				repoID, it.ID, it.Title, it.Type, it.Priority, it.Area, status,
				it.Estimate, it.Risk, it.NotBefore, it.ACTotal, it.ACChecked,
				group.archived, it.Path, now,
				epic, parallel, string(confJSON),
			); err != nil {
				return err
			}
			keep[it.ID] = struct{}{}
		}
	}

	// Garbage-collect rows for items that no longer exist on disk. Without
	// this, deleting an item file leaves its row in the items table forever
	// and `workspace status` over-reports counts indefinitely.
	rows, err := tx.QueryContext(ctx, `SELECT item_id FROM items WHERE repo_id = ?`, repoID)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if _, ok := keep[id]; !ok {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM items WHERE repo_id = ? AND item_id = ?`, repoID, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}
