// Package chat is squad's typed chat-verb layer: ask, say, fyi, milestone,
// thinking, stuck, and the durable bus that backs them. The verbs persist
// into messages rows in ~/.squad/global.db and stream over SSE for the
// dashboard; this package owns the persistence, fan-out, and dedup logic.
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
