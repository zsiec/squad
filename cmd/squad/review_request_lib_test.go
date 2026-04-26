package main

import (
	"context"
	"strings"
	"testing"
)

func TestReviewRequest_PureStoresReviewer(t *testing.T) {
	f := newChatFixture(t)
	res, err := ReviewRequest(context.Background(), ReviewRequestArgs{
		Chat:     f.chat,
		AgentID:  f.agentID,
		ItemID:   "BUG-9",
		Reviewer: "@agent-c",
	})
	if err != nil {
		t.Fatalf("ReviewRequest: %v", err)
	}
	if res == nil || res.ItemID != "BUG-9" || res.Reviewer != "agent-c" {
		t.Fatalf("unexpected result: %+v", res)
	}
	thread, _, _, mentions, _ := f.firstMessage(t)
	if thread != "BUG-9" {
		t.Fatalf("thread=%q", thread)
	}
	if !strings.Contains(mentions, "agent-c") {
		t.Fatalf("mentions=%q", mentions)
	}
}
