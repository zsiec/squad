package stats

import (
	"context"
	"testing"
)

func TestLoadVerifySeriesSurfacesQueryError(t *testing.T) {
	db := openTestDB(t)
	ensureAttestationsTable(t, db)
	if _, err := db.Exec(`DROP TABLE claim_history`); err != nil {
		t.Fatal(err)
	}
	snap := &Snapshot{Series: Series{VerificationRateDaily: []DailyRatePoint{}}}
	err := loadVerifySeries(context.Background(), db, "repo-1", 0, 0, snap)
	if err == nil {
		t.Fatal("expected error from missing claim_history table, got nil")
	}
}
