package claims

import (
	"context"
	"database/sql"
)

// rowQuerier matches the QueryRowContext method shared by *sql.DB and
// *sql.Tx, so HolderOf works inside or outside a transaction without the
// caller having to choose between two near-identical helpers.
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// HolderOf returns the agent_id currently holding the claim on
// (repoID, itemID). When no claim row exists the call returns
// "" + sql.ErrNoRows so callers can distinguish "unclaimed" from a real
// query failure; other DB errors surface unchanged.
func HolderOf(ctx context.Context, q rowQuerier, repoID, itemID string) (string, error) {
	var agent string
	err := q.QueryRowContext(ctx,
		`SELECT agent_id FROM claims WHERE repo_id = ? AND item_id = ?`,
		repoID, itemID).Scan(&agent)
	return agent, err
}
