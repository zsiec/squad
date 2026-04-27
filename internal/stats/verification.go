package stats

import (
	"context"
	"database/sql"
)

func computeVerification(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	snap.Verification.ByKind = map[string]VerificationKindRow{}
	if !tableExists(ctx, db, "attestations") {
		return nil
	}
	required := []string{"test", "review"}

	// Dones in window.
	donesArgs := append(scopeArgs(repoID), since, until, until)
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM claim_history
		WHERE `+scopeSQL("", repoID)+` AND outcome = 'done'
		  AND released_at >= ? AND (? = 0 OR released_at < ?)`,
		donesArgs...).
		Scan(&snap.Verification.DonesTotal); err != nil {
		return err
	}

	// Dones whose required kinds are all attested with exit=0.
	fq := `SELECT COUNT(DISTINCT ch.item_id) FROM claim_history ch
		WHERE ` + scopeSQL("ch.", repoID) + ` AND ch.outcome = 'done'
		  AND ch.released_at >= ? AND (? = 0 OR ch.released_at < ?)`
	fArgs := append(scopeArgs(repoID), since, until, until)
	for _, k := range required {
		fq += ` AND EXISTS (SELECT 1 FROM attestations a
			WHERE a.repo_id = ch.repo_id AND a.item_id = ch.item_id
			AND a.kind = ? AND a.exit_code = 0)`
		fArgs = append(fArgs, k)
	}
	if err := db.QueryRowContext(ctx, fq, fArgs...).Scan(&snap.Verification.DonesWithFullEvidence); err != nil {
		return err
	}
	if snap.Verification.DonesTotal > 0 {
		r := float64(snap.Verification.DonesWithFullEvidence) / float64(snap.Verification.DonesTotal)
		snap.Verification.Rate = &r
	}

	// Per-kind attested + passed counts.
	kindArgs := append(scopeArgs(repoID), since, until, until)
	kr, err := db.QueryContext(ctx, `
		SELECT kind, COUNT(*),
		       SUM(CASE WHEN exit_code = 0 THEN 1 ELSE 0 END)
		FROM attestations
		WHERE `+scopeSQL("", repoID)+` AND created_at >= ? AND (? = 0 OR created_at < ?)
		GROUP BY kind`, kindArgs...)
	if err != nil {
		return err
	}
	defer kr.Close()
	for kr.Next() {
		var kind string
		var attested, passed int64
		if err := kr.Scan(&kind, &attested, &passed); err != nil {
			return err
		}
		snap.Verification.ByKind[kind] = VerificationKindRow{Attested: attested, Passed: passed}
	}

	// Reviewer disagreement.
	var total, withD int64
	revArgs := append(scopeArgs(repoID), since, until, until)
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN review_disagreements > 0 THEN 1 ELSE 0 END), 0)
		FROM attestations
		WHERE `+scopeSQL("", repoID)+` AND kind = 'review'
		  AND created_at >= ? AND (? = 0 OR created_at < ?)`,
		revArgs...).Scan(&total, &withD); err != nil {
		return err
	}
	snap.Verification.ReviewsTotal = total
	snap.Verification.ReviewsWithDisagreement = withD
	if total > 0 {
		r := float64(withD) / float64(total)
		snap.Verification.ReviewerDisagreementRate = &r
	}
	return nil
}

func tableExists(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
		name).Scan(&n)
	return n > 0
}
