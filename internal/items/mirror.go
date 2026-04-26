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

	now := time.Now().Unix()
	keep := make(map[string]struct{}, len(w.Active)+len(w.Done))
	for _, group := range []struct {
		items    []Item
		archived bool
	}{
		{w.Active, false},
		{w.Done, true},
	} {
		for _, it := range group.items {
			if err := PersistOne(ctx, tx, repoID, it, group.archived, now); err != nil {
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
