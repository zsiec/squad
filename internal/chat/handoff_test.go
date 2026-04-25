package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandoffBody_EmptyRejected(t *testing.T) {
	c, _ := newTestChat(t)
	if err := c.PostHandoff(context.Background(), "agent-a", HandoffBody{}); err == nil {
		t.Fatal("expected error for empty handoff")
	}
}

func TestHandoffBody_StoresJSONWithKindHandoff(t *testing.T) {
	c, db := newTestChat(t)
	body := HandoffBody{
		Shipped:  []string{"BUG-1"},
		InFlight: []string{"BUG-2"},
		Note:     "running tests then back",
	}
	if err := c.PostHandoff(context.Background(), "agent-a", body); err != nil {
		t.Fatal(err)
	}

	var kind, raw string
	_ = db.QueryRow(`SELECT kind, body FROM messages`).Scan(&kind, &raw)
	if kind != KindHandoff {
		t.Fatalf("kind=%q", kind)
	}
	var got HandoffBody
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Shipped) != 1 || got.Shipped[0] != "BUG-1" {
		t.Fatalf("shipped=%v", got.Shipped)
	}
}

func TestHandoffBody_SummaryFormatted(t *testing.T) {
	body := HandoffBody{Shipped: []string{"a", "b"}, Unblocks: []string{"c"}}
	got := body.Summary()
	if !strings.Contains(got, "shipped 2") {
		t.Fatalf("summary=%q", got)
	}
	if !strings.Contains(got, "unblocks c") {
		t.Fatalf("summary=%q", got)
	}
}
