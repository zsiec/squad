package chat

import (
	"context"
	"testing"
)

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		now, last int64
		want      string
	}{
		{1000, 0, "offline"},
		{1000, 999, "active"},
		{1000, 1000 - 4*60, "active"},
		{1000, 1000 - 10*60, "idle"},
		{1000, 1000 - 60*60, "stale"},
		{1000, 1000 - 24*60*60, "offline"},
	}
	for _, tc := range cases {
		if got := DeriveStatus(tc.now, tc.last); got != tc.want {
			t.Errorf("DeriveStatus(now=%d, last=%d) = %q want %q",
				tc.now, tc.last, got, tc.want)
		}
	}
}

func TestWhoList_ReturnsAgentsAndDerivesStatus(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()
	if err := registerTestAgent(ctx, db, "repo-test", "agent-b", "B", c.nowUnix()); err != nil {
		t.Fatal(err)
	}

	rows, err := c.WhoList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 agents, got %d", len(rows))
	}
}
