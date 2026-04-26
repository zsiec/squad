package client

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// SubscribeEvents opens an SSE connection to /api/events and returns a
// channel of parsed Event values. The connection is reestablished with
// exponential backoff (1s, 2s, 4s, ..., capped at 30s) until ctx is done.
// The returned channel closes when ctx is canceled.
func (c *Client) SubscribeEvents(ctx context.Context) <-chan Event {
	out := make(chan Event, 64)
	go func() {
		defer close(out)
		backoff := time.Second
		for ctx.Err() == nil {
			err := c.streamEvents(ctx, out)
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				backoff = time.Second
				continue
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}()
	return out
}

func (c *Client) streamEvents(ctx context.Context, out chan<- Event) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/events", nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &StatusErr{Code: resp.StatusCode, Endpoint: "GET /api/events"}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var ev Event
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, ":"):
			continue
		case strings.HasPrefix(line, "event: "):
			ev.Kind = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			ev.Payload = json.RawMessage(strings.TrimPrefix(line, "data: "))
		case line == "":
			if ev.Kind != "" {
				select {
				case out <- ev:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			ev = Event{}
		}
	}
	return scanner.Err()
}
