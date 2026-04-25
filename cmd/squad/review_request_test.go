package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunReviewRequest_RequiresItem(t *testing.T) {
	f := newChatFixture(t)
	if code := runReviewRequestBody(context.Background(), f.chat, f.agentID, "", "", io.Discard); code == 0 {
		t.Fatal("expected non-zero on missing item")
	}
}

func TestRunReviewRequest_WithMentionStoresReviewer(t *testing.T) {
	f := newChatFixture(t)
	if code := runReviewRequestBody(context.Background(), f.chat, f.agentID, "BUG-9", "@agent-c", io.Discard); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	thread, _, _, mentions, _ := f.firstMessage(t)
	if thread != "BUG-9" {
		t.Fatalf("thread=%q", thread)
	}
	if !strings.Contains(mentions, "agent-c") {
		t.Fatalf("mentions=%q", mentions)
	}
}
