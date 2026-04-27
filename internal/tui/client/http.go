// Package client is the typed HTTP+SSE client the TUI uses to talk to
// squad serve. No view module imports anything outside this package
// (enforced by TestImportBoundary in Phase B Task 27).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	agent   string
	http    *http.Client
}

type StatusErr struct {
	Code     int
	Body     string
	Endpoint string
}

func (e *StatusErr) Error() string {
	return fmt.Sprintf("%s -> %d: %s", e.Endpoint, e.Code, e.Body)
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) WithAgent(agent string) *Client {
	c.agent = agent
	return c
}

func (c *Client) GET(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) POST(ctx context.Context, path string, body any, out any) error {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(b)
	}
	return c.do(ctx, http.MethodPost, path, buf, out)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if c.agent != "" {
		req.Header.Set("X-Squad-Agent", c.agent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &StatusErr{Code: resp.StatusCode, Body: string(raw), Endpoint: method + " " + path}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
