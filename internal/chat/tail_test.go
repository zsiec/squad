package chat

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTail_OneShotPrintsMessages(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "hello"})

	var buf bytes.Buffer
	if err := c.TailOnce(ctx, &buf, ThreadGlobal, 0, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("missing body: %q", buf.String())
	}
}

func TestTail_FilterByThread(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "g-msg"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: "BUG-1", Kind: KindFYI, Body: "b-msg"})

	var buf bytes.Buffer
	if err := c.TailOnce(ctx, &buf, "BUG-1", 0, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "g-msg") {
		t.Fatalf("filter leaked: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "b-msg") {
		t.Fatalf("filter missed: %q", buf.String())
	}
}

func TestTail_FilterByKind(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindSay, Body: "say-msg"})
	_ = c.Post(ctx, PostRequest{AgentID: "agent-a", Thread: ThreadGlobal, Kind: KindFYI, Body: "fyi-msg"})

	var buf bytes.Buffer
	if err := c.TailOnce(ctx, &buf, "all", 0, []string{KindFYI}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "say-msg") {
		t.Fatalf("kind filter leaked: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "fyi-msg") {
		t.Fatalf("kind filter missed: %q", buf.String())
	}
}

func TestParseSince(t *testing.T) {
	cases := []struct {
		in  string
		err bool
	}{
		{"", false},
		{"30m", false},
		{"2h", false},
		{"7d", false},
		{"junk", true},
	}
	for _, tc := range cases {
		_, err := ParseSince(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("ParseSince(%q) err=%v want err=%v", tc.in, err, tc.err)
		}
	}
}
