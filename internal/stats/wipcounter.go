package stats

import (
	"context"
	"database/sql"
	"time"
)

func RecordWIPViolation(ctx context.Context, db *sql.DB, repoID, agentID string, heldAtAttempt, capAtAttempt int64) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO wip_violations (repo_id, agent_id, attempted_at, held_at_attempt, cap_at_attempt)
		VALUES (?, ?, ?, ?, ?)`,
		repoID, agentID, time.Now().Unix(), heldAtAttempt, capAtAttempt)
	return err
}

// CountWIPViolations returns the total wip_violations rows in the time
// window [since, until]. since=0, until=0 means unbounded.
func CountWIPViolations(ctx context.Context, db *sql.DB, repoID string, since, until int64) (int64, error) {
	return countViolations(ctx, db,
		`SELECT COUNT(*) FROM wip_violations WHERE repo_id = ?`,
		[]any{repoID}, since, until)
}

func CountWIPViolationsByAgent(ctx context.Context, db *sql.DB, repoID, agentID string, since, until int64) (int64, error) {
	return countViolations(ctx, db,
		`SELECT COUNT(*) FROM wip_violations WHERE repo_id = ? AND agent_id = ?`,
		[]any{repoID, agentID}, since, until)
}

func countViolations(ctx context.Context, db *sql.DB, q string, args []any, since, until int64) (int64, error) {
	if since > 0 {
		q += ` AND attempted_at >= ?`
		args = append(args, since)
	}
	if until > 0 {
		q += ` AND attempted_at < ?`
		args = append(args, until)
	}
	var n int64
	err := db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}
