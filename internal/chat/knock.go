package chat

import (
	"context"
	"fmt"
)

func (c *Chat) Knock(ctx context.Context, agentID, target, body string) error {
	if target == "" {
		return fmt.Errorf("knock: target required")
	}
	full := fmt.Sprintf("@%s KNOCK: %s", target, body)
	return c.Post(ctx, PostRequest{
		AgentID:  agentID,
		Thread:   ThreadGlobal,
		Kind:     KindKnock,
		Body:     full,
		Mentions: []string{target},
		Priority: PriorityHigh,
	})
}
