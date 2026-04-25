package chat

import (
	"context"
	"fmt"
)

func (c *Chat) ReviewRequest(ctx context.Context, agentID, itemID, reviewer string) error {
	if itemID == "" {
		return fmt.Errorf("review-request: item id required")
	}
	body := fmt.Sprintf("review requested on %s", itemID)
	var mentions []string
	if reviewer != "" {
		body = fmt.Sprintf("@%s review requested on %s", reviewer, itemID)
		mentions = []string{reviewer}
	}
	return c.Post(ctx, PostRequest{
		AgentID:  agentID,
		Thread:   itemID,
		Kind:     KindReviewReq,
		Body:     body,
		Mentions: mentions,
	})
}
