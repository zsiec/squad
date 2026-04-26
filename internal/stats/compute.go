package stats

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ComputeOpts struct {
	RepoID string
	Now    time.Time
	Window time.Duration
}

func Compute(ctx context.Context, db *sql.DB, opts ComputeOpts) (Snapshot, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	until := opts.Now.Unix()
	var since int64
	label := "all"
	if opts.Window > 0 {
		since = opts.Now.Add(-opts.Window).Unix()
		label = opts.Window.String()
	}
	snap := Snapshot{
		SchemaVersion: CurrentSchemaVersion,
		GeneratedAt:   until, RepoID: opts.RepoID,
		Window: Window{Since: since, Until: until, Label: label},
	}
	steps := []struct {
		name string
		fn   func(context.Context, *sql.DB, string, int64, int64, *Snapshot) error
	}{
		{"items", computeItems2},
		{"claims", computeClaims},
		{"verification", computeVerification},
		{"learnings", computeLearnings},
		{"tokens", computeTokens},
		{"by_agent", computeByAgent},
		{"by_epic", computeByEpic},
		{"series", computeSeries},
	}
	for _, st := range steps {
		if err := st.fn(ctx, db, opts.RepoID, since, until, &snap); err != nil {
			return snap, fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return snap, nil
}

// computeItems2 ignores since/until to fit the step signature.
func computeItems2(ctx context.Context, db *sql.DB, repoID string, _, _ int64, snap *Snapshot) error {
	return computeItems(ctx, db, repoID, snap)
}

func computeItems(ctx context.Context, db *sql.DB, repoID string, snap *Snapshot) error {
	snap.Items.ByPriority = map[string]int64{}
	snap.Items.ByArea = map[string]int64{}
	rows, err := db.QueryContext(ctx, `
		SELECT i.status, COALESCE(i.priority,''), COALESCE(i.area,''),
		       (CASE WHEN c.item_id IS NULL THEN 0 ELSE 1 END) AS claimed
		FROM items i
		LEFT JOIN claims c ON c.item_id = i.item_id AND c.repo_id = i.repo_id
		WHERE i.repo_id = ? AND i.archived = 0`, repoID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var status, priority, area string
		var claimed int
		if err := rows.Scan(&status, &priority, &area, &claimed); err != nil {
			return err
		}
		snap.Items.Total++
		switch {
		case claimed == 1 && (status == "open" || status == ""):
			snap.Items.Claimed++
		case status == "blocked":
			snap.Items.Blocked++
		case status == "done":
			snap.Items.Done++
		default:
			snap.Items.Open++
		}
		if priority != "" {
			snap.Items.ByPriority[priority]++
		}
		if area != "" {
			snap.Items.ByArea[area]++
		}
	}
	return rows.Err()
}

func computeClaims(ctx context.Context, db *sql.DB, repoID string, since, until int64, snap *Snapshot) error {
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM claims WHERE repo_id = ?`, repoID).
		Scan(&snap.Claims.Active); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT released_at - claimed_at FROM claim_history
		WHERE repo_id = ? AND outcome = 'done'
		  AND released_at >= ? AND (? = 0 OR released_at < ?)`,
		repoID, since, until, until)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ds []float64
	for rows.Next() {
		var d int64
		if err := rows.Scan(&d); err != nil {
			return err
		}
		ds = append(ds, float64(d))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	snap.Claims.CompletedInWindow = int64(len(ds))
	snap.Claims.DurationSeconds = computePercentiles(ds)
	// Wall-time-to-done equals duration until R3 splits "first-claim → final-done".
	snap.Claims.WallTimeToDoneSeconds = computePercentiles(ds)
	snap.Claims.WIPViolationsAttempted, _ = CountWIPViolations(ctx, db, repoID, since, until)
	return nil
}
