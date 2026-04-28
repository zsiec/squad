package stats

import (
	"context"
	"testing"
	"time"
)

func TestTopCloser_PicksWinnerByCount(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	in := now.Add(-3 * time.Hour).Unix()

	seedItem(t, db, "FEAT-1", "done", "P2", "chat")
	seedItem(t, db, "FEAT-2", "done", "P2", "chat")
	seedItem(t, db, "FEAT-3", "done", "P2", "chat")
	seedItem(t, db, "FEAT-4", "done", "P2", "chat")
	seedItem(t, db, "FEAT-5", "done", "P2", "stats") // different area
	seedClaimHistory(t, db, "FEAT-1", "agent-a", in-100, in, "done")
	seedClaimHistory(t, db, "FEAT-2", "agent-a", in-100, in+1, "done")
	seedClaimHistory(t, db, "FEAT-3", "agent-a", in-100, in+2, "done")
	seedClaimHistory(t, db, "FEAT-4", "agent-b", in-100, in+3, "done")
	seedClaimHistory(t, db, "FEAT-5", "agent-b", in-100, in+4, "done")

	row, ok, err := TopCloser(context.Background(), db, "repo-1", "chat", now.Add(-30*24*time.Hour).Unix(), now.Unix(), 3)
	if err != nil {
		t.Fatalf("TopCloser: %v", err)
	}
	if !ok {
		t.Fatal("expected a top closer, got ok=false")
	}
	if row.AgentID != "agent-a" {
		t.Errorf("AgentID = %q; want agent-a", row.AgentID)
	}
	if row.DoneCount != 3 {
		t.Errorf("DoneCount = %d; want 3", row.DoneCount)
	}
}

func TestTopCloser_BelowThresholdSuppressed(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	in := now.Add(-3 * time.Hour).Unix()

	seedItem(t, db, "FEAT-1", "done", "P2", "chat")
	seedItem(t, db, "FEAT-2", "done", "P2", "chat")
	seedClaimHistory(t, db, "FEAT-1", "agent-a", in-100, in, "done")
	seedClaimHistory(t, db, "FEAT-2", "agent-a", in-100, in+1, "done")

	_, ok, err := TopCloser(context.Background(), db, "repo-1", "chat", now.Add(-30*24*time.Hour).Unix(), now.Unix(), 3)
	if err != nil {
		t.Fatalf("TopCloser: %v", err)
	}
	if ok {
		t.Error("expected ok=false (only 2 closes, threshold 3)")
	}
}

func TestTopCloser_TiebreakerMostRecentWins(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	base := now.Add(-3 * time.Hour).Unix()

	for i := 1; i <= 6; i++ {
		seedItem(t, db, fmtID(i), "done", "P2", "chat")
	}
	// Both agents have 3 closes in chat. agent-b's most recent is later.
	seedClaimHistory(t, db, "FEAT-1", "agent-a", base-100, base+10, "done")
	seedClaimHistory(t, db, "FEAT-2", "agent-a", base-100, base+20, "done")
	seedClaimHistory(t, db, "FEAT-3", "agent-a", base-100, base+30, "done")
	seedClaimHistory(t, db, "FEAT-4", "agent-b", base-100, base+15, "done")
	seedClaimHistory(t, db, "FEAT-5", "agent-b", base-100, base+25, "done")
	seedClaimHistory(t, db, "FEAT-6", "agent-b", base-100, base+40, "done")

	row, ok, err := TopCloser(context.Background(), db, "repo-1", "chat", now.Add(-30*24*time.Hour).Unix(), now.Unix(), 3)
	if err != nil || !ok {
		t.Fatalf("TopCloser err=%v ok=%v", err, ok)
	}
	if row.AgentID != "agent-b" {
		t.Errorf("AgentID = %q; want agent-b (more-recent close wins tiebreak)", row.AgentID)
	}
}

func TestTopCloser_WindowScoping(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	in := now.Add(-3 * time.Hour).Unix()
	out := now.Add(-48 * time.Hour).Unix()

	seedItem(t, db, "FEAT-1", "done", "P2", "chat")
	seedItem(t, db, "FEAT-2", "done", "P2", "chat")
	seedItem(t, db, "FEAT-3", "done", "P2", "chat")
	seedClaimHistory(t, db, "FEAT-1", "agent-a", in-100, in, "done")
	seedClaimHistory(t, db, "FEAT-2", "agent-a", out-100, out, "done")
	seedClaimHistory(t, db, "FEAT-3", "agent-a", out-200, out-50, "done")

	_, ok, err := TopCloser(context.Background(), db, "repo-1", "chat", now.Add(-24*time.Hour).Unix(), now.Unix(), 3)
	if err != nil {
		t.Fatalf("TopCloser: %v", err)
	}
	if ok {
		t.Error("expected ok=false (only 1 done in 24h window)")
	}
}

func TestTopCloser_ReleasedNotCounted(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	in := now.Add(-3 * time.Hour).Unix()

	seedItem(t, db, "FEAT-1", "done", "P2", "chat")
	seedItem(t, db, "FEAT-2", "open", "P2", "chat")
	seedItem(t, db, "FEAT-3", "open", "P2", "chat")
	seedItem(t, db, "FEAT-4", "open", "P2", "chat")
	seedClaimHistory(t, db, "FEAT-1", "agent-a", in-100, in, "done")
	seedClaimHistory(t, db, "FEAT-2", "agent-a", in-100, in+1, "released")
	seedClaimHistory(t, db, "FEAT-3", "agent-a", in-100, in+2, "released")
	seedClaimHistory(t, db, "FEAT-4", "agent-a", in-100, in+3, "released")

	_, ok, err := TopCloser(context.Background(), db, "repo-1", "chat", now.Add(-30*24*time.Hour).Unix(), now.Unix(), 3)
	if err != nil {
		t.Fatalf("TopCloser: %v", err)
	}
	if ok {
		t.Error("expected ok=false (only 1 done; 'released' rows must not count)")
	}
}

func fmtID(n int) string {
	if n < 10 {
		return "FEAT-" + string(rune('0'+n))
	}
	return "FEAT-1" + string(rune('0'+n-10))
}
