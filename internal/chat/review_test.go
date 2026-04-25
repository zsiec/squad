package chat

import (
	"context"
	"strings"
	"testing"
)

func TestReviewRequest_StoresInItemThread(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.ReviewRequest(ctx, "agent-a", "BUG-300", "agent-c"); err != nil {
		t.Fatal(err)
	}

	var thread, kind, mentions string
	_ = db.QueryRow(`SELECT thread, kind, mentions FROM messages`).Scan(&thread, &kind, &mentions)
	if thread != "BUG-300" {
		t.Fatalf("thread=%q", thread)
	}
	if kind != KindReviewReq {
		t.Fatalf("kind=%q", kind)
	}
	if !strings.Contains(mentions, "agent-c") {
		t.Fatalf("mentions=%q", mentions)
	}
}

func TestReviewRequest_NoReviewerOmitsMention(t *testing.T) {
	c, db := newTestChat(t)
	if err := c.ReviewRequest(context.Background(), "agent-a", "BUG-301", ""); err != nil {
		t.Fatal(err)
	}
	var mentions string
	_ = db.QueryRow(`SELECT mentions FROM messages`).Scan(&mentions)
	if mentions != "[]" {
		t.Fatalf("mentions=%q", mentions)
	}
}
