// Package chat is squad's typed chat-verb layer: ask, say, fyi, milestone,
// thinking, stuck, knock, answer, and the durable bus that backs them. The
// verbs persist into messages rows in ~/.squad/global.db and stream over
// SSE for the dashboard; this package owns the persistence, fan-out, and
// dedup logic.
package chat

import (
	"context"
	"fmt"
)

func (c *Chat) Ask(ctx context.Context, agentID, thread, target, body string) error {
	if target == "" {
		return fmt.Errorf("ask: target agent required")
	}
	full := fmt.Sprintf("@%s %s", target, body)
	return c.Post(ctx, PostRequest{
		AgentID:  agentID,
		Thread:   thread,
		Kind:     KindAsk,
		Body:     full,
		Mentions: []string{target},
	})
}

func (c *Chat) Answer(ctx context.Context, agentID string, ref int64, body string) error {
	// Validate ref points at an existing message in this repo. Without this,
	// `squad answer 99999 ...` happily stored 're:99999 <body>' even when no
	// such message id existed.
	var threadOfRef string
	err := c.db.QueryRowContext(ctx,
		`SELECT thread FROM messages WHERE id = ? AND repo_id = ?`,
		ref, c.repoID).Scan(&threadOfRef)
	if err != nil {
		return fmt.Errorf("answer: no message with id=%d in this repo", ref)
	}
	full := fmt.Sprintf("re:%d %s", ref, body)
	return c.Post(ctx, PostRequest{
		AgentID: agentID,
		Thread:  threadOfRef,
		Kind:    KindAnswer,
		Body:    full,
	})
}
