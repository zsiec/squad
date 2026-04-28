package stats

import (
	"context"
	"database/sql"
)

type CloserRow struct {
	AgentID     string
	DisplayName string
	DoneCount   int64
	LastCloseAt int64
}

// TopCloser returns the agent with the most done-closes in `area` over the
// window [since, until). ok=false when no agent meets minCloses. Tiebreaker:
// most-recent close wins; then display_name asc. until=0 disables the upper
// bound (matches the convention used elsewhere in this package).
func TopCloser(ctx context.Context, db *sql.DB, repoID, area string, since, until int64, minCloses int) (CloserRow, bool, error) {
	args := append(scopeArgs(repoID), area, since, until, until, int64(minCloses))
	q := `
		SELECT ch.agent_id, COALESCE(a.display_name, ch.agent_id),
		       COUNT(*) AS dc,
		       MAX(ch.released_at) AS lc
		FROM claim_history ch
		INNER JOIN items i ON i.repo_id = ch.repo_id AND i.item_id = ch.item_id
		LEFT JOIN agents a ON a.id = ch.agent_id
		WHERE ` + scopeSQL("ch.", repoID) + ` AND ch.outcome = 'done'
		  AND COALESCE(i.area, '') = ?
		  AND ch.released_at >= ? AND (? = 0 OR ch.released_at < ?)
		GROUP BY ch.agent_id
		HAVING dc >= ?
		ORDER BY dc DESC, lc DESC, COALESCE(a.display_name, ch.agent_id) ASC
		LIMIT 1`
	var row CloserRow
	err := db.QueryRowContext(ctx, q, args...).Scan(&row.AgentID, &row.DisplayName, &row.DoneCount, &row.LastCloseAt)
	if err == sql.ErrNoRows {
		return CloserRow{}, false, nil
	}
	if err != nil {
		return CloserRow{}, false, err
	}
	return row, true, nil
}
