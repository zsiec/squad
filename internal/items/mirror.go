package items

import (
	"context"
	"database/sql"
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
                   risk, not_before, ac_total, ac_checked, archived, path, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(repo_id, item_id) DO UPDATE SET
  title=excluded.title, type=excluded.type, priority=excluded.priority,
  area=excluded.area, status=excluded.status, estimate=excluded.estimate,
  risk=excluded.risk, not_before=excluded.not_before, ac_total=excluded.ac_total,
  ac_checked=excluded.ac_checked, archived=excluded.archived, path=excluded.path,
  updated_at=excluded.updated_at
`
	now := time.Now().Unix()
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
			if _, err := tx.ExecContext(ctx, upsert,
				repoID, it.ID, it.Title, it.Type, it.Priority, it.Area, status,
				it.Estimate, it.Risk, it.NotBefore, it.ACTotal, it.ACChecked,
				group.archived, it.Path, now,
			); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
