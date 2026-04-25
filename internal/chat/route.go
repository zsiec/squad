package chat

import "context"

// ResolveThread returns the thread a chatty verb should post to.
// Priority: explicit override > current claim > global fallback.
func (c *Chat) ResolveThread(ctx context.Context, agentID, override string) string {
	if override != "" {
		return override
	}
	var item string
	_ = c.db.QueryRowContext(ctx,
		`SELECT item_id FROM claims WHERE agent_id = ? AND repo_id = ? ORDER BY claimed_at DESC LIMIT 1`,
		agentID, c.repoID).Scan(&item)
	if item == "" {
		return ThreadGlobal
	}
	return item
}
