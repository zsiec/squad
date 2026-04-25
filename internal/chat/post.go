package chat

import (
	"context"
	"encoding/json"
	"fmt"
)

type PostRequest struct {
	AgentID  string
	Thread   string
	Kind     string
	Body     string
	Mentions []string
	Priority string
}

func (c *Chat) Post(ctx context.Context, req PostRequest) error {
	if req.AgentID == "" {
		return fmt.Errorf("post: agent id required")
	}
	if req.Thread == "" {
		return fmt.Errorf("post: thread required")
	}
	if req.Kind == "" {
		return fmt.Errorf("post: kind required")
	}
	if req.Priority == "" {
		req.Priority = PriorityNormal
	}
	if req.Mentions == nil {
		req.Mentions = ParseMentions(req.Body)
	}
	if req.Mentions == nil {
		req.Mentions = []string{}
	}
	mjson, err := json.Marshal(req.Mentions)
	if err != nil {
		return err
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := c.nowUnix()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, c.repoID, now, req.AgentID, req.Thread, req.Kind, req.Body, string(mjson), req.Priority)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx,
		`UPDATE agents SET last_tick_at = ?, status = 'active' WHERE id = ?`,
		now, req.AgentID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	c.bus.Publish(Event{
		Kind: "message",
		Payload: map[string]any{
			"id":       id,
			"thread":   req.Thread,
			"kind":     req.Kind,
			"agent_id": req.AgentID,
			"body":     req.Body,
		},
	})
	return nil
}
