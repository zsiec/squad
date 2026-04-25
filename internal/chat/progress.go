package chat

import (
	"context"
	"fmt"
)

func (c *Chat) ReportProgress(ctx context.Context, agentID, itemID string, pct int, note string) error {
	if pct < 0 || pct > 100 {
		return fmt.Errorf("progress must be 0..100, got %d", pct)
	}
	if itemID == "" {
		return fmt.Errorf("progress: item id required")
	}
	now := c.nowUnix()
	if _, err := c.db.ExecContext(ctx, `
		INSERT INTO progress (item_id, pct, reported_at, reported_by, note)
		VALUES (?, ?, ?, ?, ?)
	`, itemID, pct, now, agentID, note); err != nil {
		return err
	}
	if _, err := c.db.ExecContext(ctx,
		`UPDATE agents SET last_tick_at = ?, status = 'active' WHERE id = ?`,
		now, agentID); err != nil {
		return err
	}
	if _, err := c.db.ExecContext(ctx,
		`UPDATE claims SET last_touch = ? WHERE agent_id = ? AND repo_id = ?`,
		now, agentID, c.repoID); err != nil {
		return err
	}
	c.bus.Publish(Event{
		Kind: "progress",
		Payload: map[string]any{
			"item_id": itemID, "agent_id": agentID, "pct": pct, "note": note,
		},
	})
	return nil
}

func (c *Chat) LatestProgress(ctx context.Context, itemID string) (int, string) {
	var pct int
	var note string
	_ = c.db.QueryRowContext(ctx, `
		SELECT pct, COALESCE(note, '') FROM progress
		WHERE item_id = ? ORDER BY reported_at DESC LIMIT 1
	`, itemID).Scan(&pct, &note)
	return pct, note
}
