package claims

import (
	"context"
	"strings"
	"testing"
)

func TestReassign_ReleasesAndPostsDirective(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	_ = s.Claim(ctx, "BUG-050", "agent-a", "", nil, false)

	if err := s.Reassign(ctx, "BUG-050", "agent-a", "agent-b"); err != nil {
		t.Fatalf("reassign: %v", err)
	}

	if err := s.Claim(ctx, "BUG-050", "agent-b", "taking over", nil, false); err != nil {
		t.Fatalf("agent-b should be able to claim post-reassign: %v", err)
	}

	var body, mentions string
	db.QueryRow(`SELECT body, mentions FROM messages WHERE kind='say' AND thread='global' ORDER BY id DESC LIMIT 1`).Scan(&body, &mentions)
	if !strings.Contains(body, "@agent-b") || !strings.Contains(body, "BUG-050") {
		t.Fatalf("directive body missing target or item: %q", body)
	}
	if !strings.Contains(mentions, "agent-b") {
		t.Fatalf("mentions missing target: %q", mentions)
	}
}

func TestReassign_FailsAtomicallyIfNotYourClaim(t *testing.T) {
	s, db := newTestStore(t)
	ctx := context.Background()
	_ = s.Claim(ctx, "BUG-051", "agent-a", "", nil, false)

	if err := s.Reassign(ctx, "BUG-051", "agent-c", "agent-b"); err == nil {
		t.Fatal("reassign by non-holder should error")
	}
	var live int
	db.QueryRow(`SELECT COUNT(*) FROM claims WHERE item_id='BUG-051' AND agent_id='agent-a'`).Scan(&live)
	if live != 1 {
		t.Fatalf("agent-a's claim disappeared: count=%d", live)
	}
}
