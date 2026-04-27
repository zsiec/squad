package stats

import (
	"context"
	"database/sql"
)

const dayBucket = `CAST(strftime('%s', date(released_at, 'unixepoch')) AS INTEGER)`

func computeSeries(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	snap.Series = Series{
		VerificationRateDaily: []DailyRatePoint{},
		ClaimP99Daily:         []DailyP99Point{},
		WIPViolationsDaily:    []DailyCountPoint{},
	}
	if tableExists(ctx, db, "attestations") {
		if err := loadVerifySeries(ctx, db, repoID, since, until, snap); err != nil {
			return err
		}
	}
	if err := loadP99Series(ctx, db, repoID, since, until, snap); err != nil {
		return err
	}
	return loadWIPSeries(ctx, db, repoID, since, until, snap)
}

func loadVerifySeries(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	args := append(scopeArgs(repoID), since, until, until)
	rows, err := db.QueryContext(ctx, `
		SELECT `+dayBucket+` AS bucket, COUNT(*) AS dones,
		       SUM(CASE
		           WHEN EXISTS (SELECT 1 FROM attestations a WHERE a.repo_id = ch.repo_id
		               AND a.item_id = ch.item_id AND a.kind='test' AND a.exit_code=0)
		            AND EXISTS (SELECT 1 FROM attestations a WHERE a.repo_id = ch.repo_id
		               AND a.item_id = ch.item_id AND a.kind='review' AND a.exit_code=0)
		           THEN 1 ELSE 0 END) AS full
		FROM claim_history ch
		WHERE `+scopeSQL("ch.", repoID)+` AND ch.outcome = 'done'
		  AND ch.released_at >= ? AND (? = 0 OR ch.released_at < ?)
		GROUP BY bucket ORDER BY bucket`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var b, dones, full int64
		if err := rows.Scan(&b, &dones, &full); err != nil {
			return err
		}
		rate := 0.0
		if dones > 0 {
			rate = float64(full) / float64(dones)
		}
		snap.Series.VerificationRateDaily = append(snap.Series.VerificationRateDaily,
			DailyRatePoint{BucketTS: b, Rate: rate, Count: dones})
	}
	return rows.Err()
}

func loadP99Series(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	args := append(scopeArgs(repoID), since, until, until)
	rows, err := db.QueryContext(ctx, `
		SELECT `+dayBucket+`, GROUP_CONCAT(released_at-claimed_at), COUNT(*)
		FROM claim_history
		WHERE `+scopeSQL("", repoID)+` AND outcome = 'done'
		  AND released_at >= ? AND (? = 0 OR released_at < ?)
		GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var b, count int64
		var concat sql.NullString
		if err := rows.Scan(&b, &concat, &count); err != nil {
			return err
		}
		p99 := 0.0
		if pc := computePercentiles(splitInts(concat.String)); pc.P99 != nil {
			p99 = *pc.P99
		}
		snap.Series.ClaimP99Daily = append(snap.Series.ClaimP99Daily,
			DailyP99Point{BucketTS: b, P99Seconds: p99, Count: count})
	}
	return rows.Err()
}

func loadWIPSeries(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	args := append(scopeArgs(repoID), since, until, until)
	rows, err := db.QueryContext(ctx, `
		SELECT CAST(strftime('%s', date(attempted_at, 'unixepoch')) AS INTEGER), COUNT(*)
		FROM wip_violations
		WHERE `+scopeSQL("", repoID)+`
		  AND attempted_at >= ? AND (? = 0 OR attempted_at < ?)
		GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var b, count int64
		if err := rows.Scan(&b, &count); err != nil {
			return err
		}
		snap.Series.WIPViolationsDaily = append(snap.Series.WIPViolationsDaily,
			DailyCountPoint{BucketTS: b, Count: count})
	}
	return rows.Err()
}
