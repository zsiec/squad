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
	full := fmt.Sprintf("re:%d %s", ref, body)
	return c.Post(ctx, PostRequest{
		AgentID: agentID,
		Thread:  ThreadGlobal,
		Kind:    KindAnswer,
		Body:    full,
	})
}
