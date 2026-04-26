package client

import (
	"context"
	"net/url"
	"strconv"
)

type ItemListOpts struct {
	Status string
}

type EpicListOpts struct {
	Spec string
}

type LearningListOpts struct {
	State string
	Kind  string
	Area  string
}

type MessagesOpts struct {
	Thread string
	Limit  int
	Since  int64
	Before int64
}

func appendQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

func (c *Client) Items(ctx context.Context, opts *ItemListOpts) ([]Item, error) {
	q := url.Values{}
	if opts != nil && opts.Status != "" {
		q.Set("status", opts.Status)
	}
	var out []Item
	return out, c.GET(ctx, appendQuery("/api/items", q), &out)
}

func (c *Client) Item(ctx context.Context, id string) (ItemDetail, error) {
	var out ItemDetail
	return out, c.GET(ctx, "/api/items/"+id, &out)
}

func (c *Client) Agents(ctx context.Context) ([]Agent, error) {
	var out []Agent
	return out, c.GET(ctx, "/api/agents", &out)
}

func (c *Client) Whoami(ctx context.Context) (Whoami, error) {
	var out Whoami
	return out, c.GET(ctx, "/api/whoami", &out)
}

func (c *Client) Specs(ctx context.Context) ([]Spec, error) {
	var out []Spec
	return out, c.GET(ctx, "/api/specs", &out)
}

func (c *Client) Spec(ctx context.Context, name string) (SpecDetail, error) {
	var out SpecDetail
	return out, c.GET(ctx, "/api/specs/"+name, &out)
}

func (c *Client) Epics(ctx context.Context, opts *EpicListOpts) ([]Epic, error) {
	q := url.Values{}
	if opts != nil && opts.Spec != "" {
		q.Set("spec", opts.Spec)
	}
	var out []Epic
	return out, c.GET(ctx, appendQuery("/api/epics", q), &out)
}

func (c *Client) Epic(ctx context.Context, name string) (EpicDetail, error) {
	var out EpicDetail
	return out, c.GET(ctx, "/api/epics/"+name, &out)
}

func (c *Client) Stats(ctx context.Context, windowSec int64) (Stats, error) {
	q := url.Values{}
	if windowSec > 0 {
		q.Set("window", strconv.FormatInt(windowSec, 10))
	}
	var out Stats
	return out, c.GET(ctx, appendQuery("/api/stats", q), &out)
}

func (c *Client) Learnings(ctx context.Context, opts *LearningListOpts) ([]Learning, error) {
	q := url.Values{}
	if opts != nil {
		if opts.State != "" {
			q.Set("state", opts.State)
		}
		if opts.Kind != "" {
			q.Set("kind", opts.Kind)
		}
		if opts.Area != "" {
			q.Set("area", opts.Area)
		}
	}
	var out []Learning
	return out, c.GET(ctx, appendQuery("/api/learnings", q), &out)
}

func (c *Client) Learning(ctx context.Context, slug string) (LearningDetail, error) {
	var out LearningDetail
	return out, c.GET(ctx, "/api/learnings/"+slug, &out)
}

func (c *Client) Attestations(ctx context.Context, itemID string) ([]Attestation, error) {
	var out []Attestation
	return out, c.GET(ctx, "/api/items/"+itemID+"/attestations", &out)
}

func (c *Client) Messages(ctx context.Context, opts *MessagesOpts) ([]Message, error) {
	q := url.Values{}
	if opts != nil {
		if opts.Thread != "" {
			q.Set("thread", opts.Thread)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Since > 0 {
			q.Set("since", strconv.FormatInt(opts.Since, 10))
		}
		if opts.Before > 0 {
			q.Set("before", strconv.FormatInt(opts.Before, 10))
		}
	}
	var out []Message
	return out, c.GET(ctx, appendQuery("/api/messages", q), &out)
}

func (c *Client) PostMessage(ctx context.Context, req *PostMessageReq) error {
	var resp map[string]any
	return c.POST(ctx, "/api/messages", req, &resp)
}

func (c *Client) Repos(ctx context.Context) ([]Repo, error) {
	var out []Repo
	return out, c.GET(ctx, "/api/repos", &out)
}

func (c *Client) WorkspaceStatus(ctx context.Context) (WorkspaceStatus, error) {
	var out WorkspaceStatus
	return out, c.GET(ctx, "/api/workspace/status", &out)
}
