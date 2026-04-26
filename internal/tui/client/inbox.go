package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type InboxEntry struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	CapturedBy string `json:"captured_by,omitempty"`
	CapturedAt int64  `json:"captured_at,omitempty"`
	ParentSpec string `json:"parent_spec,omitempty"`
	DoRPass    bool   `json:"dor_pass"`
	Path       string `json:"path"`
}

type InboxOpts struct{}

func (c *Client) Inbox(ctx context.Context, opts InboxOpts) ([]InboxEntry, error) {
	var out []InboxEntry
	return out, c.GET(ctx, "/api/inbox", &out)
}

type DoRViolation struct {
	Rule    string `json:"rule"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type DoRViolationsError struct {
	Violations []DoRViolation
}

func (e *DoRViolationsError) Error() string {
	return fmt.Sprintf("definition of ready failed (%d violations)", len(e.Violations))
}

func (c *Client) Accept(ctx context.Context, id string) error {
	err := c.POST(ctx, "/api/items/"+id+"/accept", map[string]any{}, nil)
	if err == nil {
		return nil
	}
	var se *StatusErr
	if errors.As(err, &se) && se.Code == http.StatusUnprocessableEntity {
		var body struct {
			Violations []DoRViolation `json:"violations"`
		}
		if jerr := json.Unmarshal([]byte(se.Body), &body); jerr == nil {
			return &DoRViolationsError{Violations: body.Violations}
		}
	}
	return err
}

func (c *Client) Reject(ctx context.Context, id, reason string) error {
	return c.POST(ctx, "/api/items/"+id+"/reject", map[string]string{"reason": reason}, nil)
}

type NewItemArgs struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Priority   string `json:"priority,omitempty"`
	Area       string `json:"area,omitempty"`
	Estimate   string `json:"estimate,omitempty"`
	Risk       string `json:"risk,omitempty"`
	Ready      bool   `json:"ready,omitempty"`
	CapturedBy string `json:"captured_by,omitempty"`
}

type ItemSummary struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Path   string `json:"path"`
}

func (c *Client) NewItem(ctx context.Context, args NewItemArgs) (*ItemSummary, error) {
	var out ItemSummary
	if err := c.POST(ctx, "/api/items", args, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
