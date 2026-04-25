package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type HandoffBody struct {
	Shipped     []string `json:"shipped,omitempty"`
	InFlight    []string `json:"in_flight,omitempty"`
	SurprisedBy []string `json:"surprised_by,omitempty"`
	Unblocks    []string `json:"unblocks,omitempty"`
	Note        string   `json:"note,omitempty"`
}

func (h HandoffBody) Empty() bool {
	return len(h.Shipped) == 0 && len(h.InFlight) == 0 &&
		len(h.SurprisedBy) == 0 && len(h.Unblocks) == 0 && h.Note == ""
}

func (h HandoffBody) Summary() string {
	var parts []string
	if len(h.Shipped) > 0 {
		parts = append(parts, fmt.Sprintf("shipped %d", len(h.Shipped)))
	}
	if len(h.InFlight) > 0 {
		parts = append(parts, fmt.Sprintf("%d in flight", len(h.InFlight)))
	}
	if len(h.Unblocks) > 0 {
		parts = append(parts, fmt.Sprintf("unblocks %s", strings.Join(h.Unblocks, ", ")))
	}
	if len(h.SurprisedBy) > 0 {
		parts = append(parts, fmt.Sprintf("%d surprises", len(h.SurprisedBy)))
	}
	if len(parts) == 0 {
		return "handoff"
	}
	return strings.Join(parts, " · ")
}

func (c *Chat) PostHandoff(ctx context.Context, agentID string, h HandoffBody) error {
	if h.Empty() {
		return fmt.Errorf("handoff body is empty (need shipped / in-flight / surprised-by / unblocks / note)")
	}
	payload, err := json.Marshal(h)
	if err != nil {
		return err
	}
	if err := c.Post(ctx, PostRequest{
		AgentID: agentID,
		Thread:  ThreadGlobal,
		Kind:    KindHandoff,
		Body:    string(payload),
	}); err != nil {
		return err
	}
	c.bus.Publish(Event{
		Kind: "handoff",
		Payload: map[string]any{
			"agent_id": agentID,
			"summary":  h.Summary(),
		},
	})
	return nil
}
