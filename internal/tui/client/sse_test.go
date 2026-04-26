package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSSE_ParsesSingleEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("event: item_changed\ndata: {\"id\":\"BUG-100\",\"kind\":\"updated\"}\n\n"))
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
		if string(ev.Payload) == "" {
			t.Fatal("empty payload")
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
		_, _ = w.Write([]byte("event: lag\ndata: {\"dropped\":3}\n\n"))
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
		_, _ = w.Write([]byte("event: item_changed\ndata: {\"id\":\"A\"}\n\n"))
		w.(http.Flusher).Flush()
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("event: message_posted\ndata: {\"id\":1}\n\n"))
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
