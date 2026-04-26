package stats

import (
	"context"
	"database/sql"
)

func computeLearnings(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	if !tableExists(ctx, db, "learnings_index") {
		return nil
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM learnings_index WHERE repo_id = ?`,
		repoID).Scan(&snap.Learnings.ApprovedTotal); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM items
		WHERE repo_id = ? AND type = 'bug' AND archived = 0
		  AND updated_at >= ? AND (? = 0 OR updated_at < ?)`,
		repoID, since, until, until).Scan(&snap.Learnings.NewBugsInWindow); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT i.item_id)
		FROM items i
		INNER JOIN learnings_index li
		  ON li.repo_id = i.repo_id AND li.area = i.area
		WHERE i.repo_id = ? AND i.type = 'bug' AND i.archived = 0
		  AND i.updated_at >= ? AND (? = 0 OR i.updated_at < ?)
		  AND li.approved_at < i.updated_at`,
		repoID, since, until, until).Scan(&snap.Learnings.RepeatMistakesInWindow); err != nil {
		return err
	}
	if snap.Learnings.NewBugsInWindow > 0 {
		r := float64(snap.Learnings.RepeatMistakesInWindow) /
			float64(snap.Learnings.NewBugsInWindow)
		snap.Learnings.RepeatMistakeRate = &r
	}
	return nil
}
