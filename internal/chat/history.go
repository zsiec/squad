package chat

import "context"

type HistoryEntry struct {
	ID    int64
	TS    int64
	Agent string
	Kind  string
	Body  string
}

func (c *Chat) History(ctx context.Context, itemID string) ([]HistoryEntry, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, ts, agent_id, kind, COALESCE(body, '') FROM messages
		WHERE thread = ? ORDER BY ts
	`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.ID, &e.TS, &e.Agent, &e.Kind, &e.Body); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}
