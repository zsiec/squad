package stats

import (
	"context"
	"database/sql"
)

func computeByAgent(_ context.Context, _ *sql.DB, _ string, _, _ int64, snap *Snapshot) error {
	snap.ByAgent = []AgentRow{}
	return nil
}

func computeByEpic(_ context.Context, _ *sql.DB, _ string, _, _ int64, snap *Snapshot) error {
	snap.ByEpic = []EpicRow{}
	return nil
}

func computeSeries(_ context.Context, _ *sql.DB, _ string, _, _ int64, snap *Snapshot) error {
	snap.Series.VerificationRateDaily = []DailyRatePoint{}
	snap.Series.ClaimP99Daily = []DailyP99Point{}
	snap.Series.WIPViolationsDaily = []DailyCountPoint{}
	return nil
}
