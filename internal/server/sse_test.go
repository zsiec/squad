package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

func TestSSE_RoundTripsPostedMessage(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-x", "X")
	s := New(db, testRepoID, Config{RepoID: testRepoID})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(150 * time.Millisecond)
		body, _ := json.Marshal(map[string]any{"thread": "global", "body": "sse-hello"})
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/messages", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Squad-Agent", "agent-x")
		resp, err := srv.Client().Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content-type=%q", got)
	}

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			continue
		}
		if strings.HasPrefix(line, "data:") && strings.Contains(line, "sse-hello") {
			return
		}
	}
	t.Fatal("did not observe message event within 3s")
}

// Regression: bus-level drops occurred but no lag event ever reached SSE.
// Two faults: lag-flush rode the 15s ping-tick (test ran shorter than
// that), and the in-band sentinel has a different JSON shape than the
// out-of-band one. Force drops by saturating the channel and keeping
// the consumer slow; assert lag arrives within the dedicated lagFlush
// interval and carries the standard Event envelope.
func TestSSE_LagEventReachesStream(t *testing.T) {
	db := newTestDB(t)
	registerAgent(t, db, "agent-x", "X")
	s := New(db, testRepoID, Config{
		RepoID:           testRepoID,
		pingInterval:     200 * time.Millisecond,
		lagFlushInterval: 50 * time.Millisecond,
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Fire-and-forget enough publishes to exceed the per-subscriber
	// buffer before the subscriber drains. The Publish loop runs faster
	// than the SSE handler can write+flush, so drops accumulate.
	go func() {
		for i := 0; i < 2500; i++ {
			s.Bus().Publish(chat.Event{
				Kind:    "message",
				Payload: map[string]any{"id": int64(i + 1)},
			})
		}
	}()

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(3 * time.Second)
	var sawLag bool
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "event: lag") {
			sawLag = true
			// Next non-blank line should be the data envelope. Both lag
			// emit paths must produce {"kind":"lag","payload":{...}}.
			data, _ := reader.ReadString('\n')
			if !strings.Contains(data, `"kind":"lag"`) || !strings.Contains(data, `"dropped":`) {
				t.Fatalf("unexpected lag data shape: %q", data)
			}
			break
		}
	}
	if !sawLag {
		t.Fatal("expected event: lag in stream within 3s of forced overflow")
	}
}

func TestSSE_PingsBeforeFirstEvent(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{pingInterval: 50 * time.Millisecond})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	reqCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, ": ping") {
			return
		}
	}
	t.Fatal("expected at least one ping comment within 400ms")
}
