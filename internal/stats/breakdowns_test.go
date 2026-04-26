package stats

import (
	"context"
	"testing"
	"time"
)

func TestByAgentSortAndCap(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(2_000_000_000, 0)
	base := now.Add(-12 * time.Hour).Unix()
	for i := 0; i < 60; i++ {
		seedClaimHistory(t, db, "BUG-X",
			"agent-"+string(rune('a'+i%30)),
			base+int64(i), base+int64(i)+100, "done")
	}
	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: now, Window: 24 * time.Hour,
	})
	if len(snap.ByAgent) > 51 {
		t.Errorf("by_agent should be capped at 50+_other; got %d", len(snap.ByAgent))
	}
	prev := int64(1 << 62)
	for _, a := range snap.ByAgent {
		if a.AgentID == "_other" {
			continue
		}
		if a.ClaimsCompleted > prev {
			t.Errorf("by_agent not sorted desc: %d > %d", a.ClaimsCompleted, prev)
		}
		prev = a.ClaimsCompleted
	}
}

func TestSeriesBucketsByDay(t *testing.T) {
	db := openTestDB(t)
	ensureAttestationsTable(t, db)
	day1 := int64(1_745_424_000)
	day2 := day1 + 86400
	for _, id := range []string{"BUG-1", "BUG-2"} {
		seedClaimHistory(t, db, id, "agent-a", day1+10, day1+100, "done")
	}
	seedAttestation(t, db, "BUG-1", "test", 0, day1+50, 0)
	seedAttestation(t, db, "BUG-1", "review", 0, day1+60, 0)
	for _, id := range []string{"BUG-3", "BUG-4"} {
		seedClaimHistory(t, db, id, "agent-a", day2+10, day2+100, "done")
	}
	seedAttestation(t, db, "BUG-3", "test", 0, day2+50, 0)
	seedAttestation(t, db, "BUG-3", "review", 0, day2+60, 0)

	snap, _ := Compute(context.Background(), db, ComputeOpts{
		RepoID: "repo-1", Now: time.Unix(day2+5*3600, 0), Window: 72 * time.Hour,
	})
	if len(snap.Series.VerificationRateDaily) != 2 {
		t.Fatalf("verify series: %d points", len(snap.Series.VerificationRateDaily))
	}
	if snap.Series.VerificationRateDaily[0].Count != 2 ||
		snap.Series.VerificationRateDaily[0].Rate != 0.5 {
		t.Errorf("day1: %+v", snap.Series.VerificationRateDaily[0])
	}
}
