package chat

import (
	"context"
	"database/sql"
)

const (
	whoActiveSec = 5 * 60
	whoIdleSec   = 30 * 60
	whoStaleSec  = 6 * 60 * 60
)

func DeriveStatus(now, lastTick int64) string {
	if lastTick == 0 {
		return "offline"
	}
	age := now - lastTick
	switch {
	case age < whoActiveSec:
		return "active"
	case age < whoIdleSec:
		return "idle"
	case age < whoStaleSec:
		return "stale"
	default:
		return "offline"
	}
}

type WhoRow struct {
	AgentID     string
	DisplayName string
	Worktree    string
	LastTick    int64
	ClaimItem   string
	Intent      string
	TouchCount  int
	Status      string
}

func (c *Chat) WhoList(ctx context.Context) ([]WhoRow, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT a.id, a.display_name, COALESCE(a.worktree, ''), a.last_tick_at,
		       COALESCE(cl.item_id, ''), COALESCE(cl.intent, ''),
		       (SELECT COUNT(*) FROM touches t WHERE t.agent_id = a.id AND t.released_at IS NULL) AS tc
		FROM agents a
		LEFT JOIN claims cl ON cl.agent_id = a.id
		WHERE a.repo_id = ?
		ORDER BY a.last_tick_at DESC
	`, c.repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := c.nowUnix()
	out := []WhoRow{}
	for rows.Next() {
		var r WhoRow
		var item, intent sql.NullString
		if err := rows.Scan(&r.AgentID, &r.DisplayName, &r.Worktree, &r.LastTick,
			&item, &intent, &r.TouchCount); err != nil {
			return nil, err
		}
		r.ClaimItem = item.String
		r.Intent = intent.String
		r.Status = DeriveStatus(now, r.LastTick)
		out = append(out, r)
	}
	return out, nil
}
