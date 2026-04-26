package chat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ResolveThread returns the thread a chatty verb should post to.
// Priority: explicit override > current claim > global fallback.
//
// A real DB error is reported to the caller — the no-claim path is
// signalled only by sql.ErrNoRows and translated to ThreadGlobal here,
// so a transient outage cannot be confused for "no current claim".
func (c *Chat) ResolveThread(ctx context.Context, agentID, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	var item string
	err := c.db.QueryRowContext(ctx,
		`SELECT item_id FROM claims WHERE agent_id = ? AND repo_id = ? ORDER BY claimed_at DESC LIMIT 1`,
		agentID, c.repoID).Scan(&item)
	if errors.Is(err, sql.ErrNoRows) {
		return ThreadGlobal, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve thread for %s: %w", agentID, err)
	}
	return item, nil
}
