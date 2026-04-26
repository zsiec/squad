package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/zsiec/squad/internal/store"
)

type PostRequest struct {
	AgentID  string
	Thread   string
	Kind     string
	Body     string
	Mentions []string
	Priority string
}

// MaxPostBodyBytes caps stored message bodies. Mirrors the server-side cap
// in internal/server/messages.go so a CLI write of a 1MB body fails the
// same way an HTTP POST of one would.
const MaxPostBodyBytes = 64 * 1024

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
	if len(req.Body) > MaxPostBodyBytes {
		return fmt.Errorf("post: body too large (%d bytes, max %d)", len(req.Body), MaxPostBodyBytes)
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

	now := c.nowUnix()
	var id int64
	err = store.WithTxRetry(ctx, c.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, c.repoID, now, req.AgentID, req.Thread, req.Kind, req.Body, string(mjson), req.Priority)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		_, err = tx.ExecContext(ctx,
			`UPDATE agents SET last_tick_at = ?, status = 'active' WHERE id = ?`,
			now, req.AgentID)
		return err
	})
	if err != nil {
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
	c.fireNotify(ctx)
	return nil
}
