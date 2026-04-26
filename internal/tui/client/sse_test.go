package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSSE_ParsesSingleEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("event: item_changed\ndata: {\"kind\":\"item_changed\",\"payload\":{\"item_id\":\"BUG-100\",\"kind\":\"claimed\"}}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := c.SubscribeEvents(ctx)
	select {
	case ev := <-ch:
		if ev.Kind != "item_changed" {
			t.Fatalf("kind=%s", ev.Kind)
		}
		var p struct {
			ItemID string `json:"item_id"`
			Kind   string `json:"kind"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("decode payload: %v (raw=%q)", err, ev.Payload)
		}
		if p.ItemID != "BUG-100" {
			t.Fatalf("payload item_id=%q", p.ItemID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestSSE_IgnoresPingComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(": ping\n\n"))
		_, _ = w.Write([]byte("event: lag\ndata: {\"kind\":\"lag\",\"payload\":{\"dropped\":3}}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := c.SubscribeEvents(ctx)
	select {
	case ev := <-ch:
		if ev.Kind != "lag" {
			t.Fatalf("expected lag (ping should be skipped), got kind=%q", ev.Kind)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout — ping comment may have been emitted as an event")
	}
}

func TestSSE_ParsesMultipleEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("event: item_changed\ndata: {\"kind\":\"item_changed\",\"payload\":{\"item_id\":\"A\"}}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("event: message_posted\ndata: {\"kind\":\"message_posted\",\"payload\":{\"id\":1}}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := c.SubscribeEvents(ctx)
	got := []string{}
	for len(got) < 2 {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatal("channel closed early")
			}
			got = append(got, ev.Kind)
			switch ev.Kind {
			case "item_changed":
				var p struct {
					ItemID string `json:"item_id"`
				}
				if err := json.Unmarshal(ev.Payload, &p); err != nil {
					t.Fatalf("decode item_changed payload: %v (raw=%q)", err, ev.Payload)
				}
				if p.ItemID != "A" {
					t.Fatalf("item_changed item_id=%q", p.ItemID)
				}
			case "message_posted":
				var p struct {
					ID int64 `json:"id"`
				}
				if err := json.Unmarshal(ev.Payload, &p); err != nil {
					t.Fatalf("decode message_posted payload: %v (raw=%q)", err, ev.Payload)
				}
				if p.ID != 1 {
					t.Fatalf("message_posted id=%d", p.ID)
				}
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout, got=%v", got)
		}
	}
	if got[0] != "item_changed" || got[1] != "message_posted" {
		t.Fatalf("got=%v", got)
	}
}

func TestSSE_AddsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		// keep open briefly so subscribe loop has time to read header
		time.Sleep(100 * time.Millisecond)
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_ = c.SubscribeEvents(ctx)
	time.Sleep(200 * time.Millisecond)
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth=%q", gotAuth)
	}
}
