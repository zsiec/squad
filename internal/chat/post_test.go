package chat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPost_StoresRow(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.Post(ctx, PostRequest{
		AgentID: "agent-a",
		Thread:  ThreadGlobal,
		Kind:    KindSay,
		Body:    "hello team",
	}); err != nil {
		t.Fatal(err)
	}

	var n int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE kind='say' AND thread='global'`).Scan(&n)
	if n != 1 {
		t.Fatalf("count=%d", n)
	}
}

func TestPost_ParsesMentionsWhenEmpty(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.Post(ctx, PostRequest{
		AgentID: "agent-a",
		Thread:  ThreadGlobal,
		Kind:    KindSay,
		Body:    "@agent-b heads up",
	}); err != nil {
		t.Fatal(err)
	}

	var mentions string
	_ = db.QueryRowContext(ctx,
		`SELECT mentions FROM messages WHERE kind='say'`).Scan(&mentions)
	if mentions != `["agent-b"]` {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestPost_ExplicitMentionsTakePrecedence(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.Post(ctx, PostRequest{
		AgentID:  "agent-a",
		Thread:   ThreadGlobal,
		Kind:     KindSay,
		Body:     "no inline @ here",
		Mentions: []string{"forced"},
	}); err != nil {
		t.Fatal(err)
	}

	var mentions string
	_ = db.QueryRowContext(ctx, `SELECT mentions FROM messages`).Scan(&mentions)
	if mentions != `["forced"]` {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestPost_PublishesEvent(t *testing.T) {
	c, _ := newTestChat(t)
	sub := c.Bus().Subscribe()
	defer c.Bus().Unsubscribe(sub)

	if err := c.Post(context.Background(), PostRequest{
		AgentID: "agent-a", Thread: "BUG-1", Kind: KindThinking, Body: "msg",
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case e := <-sub:
		if e.Kind != "message" {
			t.Fatalf("event kind=%q", e.Kind)
		}
		if e.Payload["thread"] != "BUG-1" || e.Payload["kind"] != KindThinking {
			t.Fatalf("payload=%v", e.Payload)
		}
	default:
		t.Fatal("expected event")
	}
}

func TestPost_BumpsHeartbeat(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	var before int64
	_ = db.QueryRowContext(ctx, `SELECT last_tick_at FROM agents WHERE id='agent-a'`).Scan(&before)

	// Advance the clock so the bump is observable.
	c.now = func() time.Time { return time.Unix(before+60, 0) }

	if err := c.Post(ctx, PostRequest{
		AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "ping",
	}); err != nil {
		t.Fatal(err)
	}

	var after int64
	_ = db.QueryRowContext(ctx, `SELECT last_tick_at FROM agents WHERE id='agent-a'`).Scan(&after)
	if after <= before {
		t.Fatalf("heartbeat not bumped: before=%d after=%d", before, after)
	}
}

func TestPost_FiresNotifyWake(t *testing.T) {
	c, _ := newTestChat(t)
	var fired atomic.Int64
	var lastRepo atomic.Value
	c.SetNotifier(func(ctx context.Context, repoID string) {
		fired.Add(1)
		lastRepo.Store(repoID)
	})

	if err := c.Post(context.Background(), PostRequest{
		AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "hi",
	}); err != nil {
		t.Fatal(err)
	}

	if fired.Load() != 1 {
		t.Fatalf("notifier should fire exactly once, got %d", fired.Load())
	}
	if got, _ := lastRepo.Load().(string); got != "repo-test" {
		t.Fatalf("notifier called with repo=%q want repo-test", got)
	}
}

func TestPost_NotifierAbsentIsNoOp(t *testing.T) {
	c, _ := newTestChat(t)
	if err := c.Post(context.Background(), PostRequest{
		AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "hi",
	}); err != nil {
		t.Fatalf("post must succeed without a notifier: %v", err)
	}
}
