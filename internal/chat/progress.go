package chat

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zsiec/squad/internal/store"
)

// ReportProgress writes the canonical progress row, bumps the agent
// heartbeat + claim last_touch, and emits a chat-stream message for the
// SSE pump — all in a single transaction so the four sites cannot
// diverge under partial failure. Bus event fires only after the
// transaction commits.
func (c *Chat) ReportProgress(ctx context.Context, agentID, itemID string, pct int, note string) error {
	if pct < 0 || pct > 100 {
		return fmt.Errorf("progress must be 0..100, got %d", pct)
	}
	if itemID == "" {
		return fmt.Errorf("progress: item id required")
	}
	now := c.nowUnix()
	body := fmt.Sprintf("%d%%", pct)
	if note != "" {
		body = body + " — " + note
	}
	err := store.WithTxRetry(ctx, c.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO progress (item_id, pct, reported_at, reported_by, note)
			VALUES (?, ?, ?, ?, ?)
		`, itemID, pct, now, agentID, note); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE agents SET last_tick_at = ?, status = 'active' WHERE id = ?`,
			now, agentID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE claims SET last_touch = ? WHERE agent_id = ? AND repo_id = ?`,
			now, agentID, c.repoID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
			VALUES (?, ?, ?, ?, 'progress', ?, '', 'normal')
		`, c.repoID, now, agentID, itemID, body)
		return err
	})
	if err != nil {
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
