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
		`SELECT COUNT(*) FROM learnings_index WHERE `+scopeSQL("", repoID),
		scopeArgs(repoID)...).Scan(&snap.Learnings.ApprovedTotal); err != nil {
		return err
	}
	bugArgs := append(scopeArgs(repoID), since, until, until)
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM items
		WHERE `+scopeSQL("", repoID)+` AND type = 'bug' AND archived = 0
		  AND updated_at >= ? AND (? = 0 OR updated_at < ?)`,
		bugArgs...).Scan(&snap.Learnings.NewBugsInWindow); err != nil {
		return err
	}
	repeatArgs := append(scopeArgs(repoID), since, until, until)
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT i.item_id)
		FROM items i
		INNER JOIN learnings_index li
		  ON li.repo_id = i.repo_id AND li.area = i.area
		WHERE `+scopeSQL("i.", repoID)+` AND i.type = 'bug' AND i.archived = 0
		  AND i.updated_at >= ? AND (? = 0 OR i.updated_at < ?)
		  AND li.approved_at < i.updated_at`,
		repeatArgs...).Scan(&snap.Learnings.RepeatMistakesInWindow); err != nil {
		return err
	}
	if snap.Learnings.NewBugsInWindow > 0 {
		r := float64(snap.Learnings.RepeatMistakesInWindow) /
			float64(snap.Learnings.NewBugsInWindow)
		snap.Learnings.RepeatMistakeRate = &r
	}
	return nil
}
