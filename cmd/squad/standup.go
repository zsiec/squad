package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
)

type standupReport struct {
	Agent          string                `json:"agent"`
	Repo           string                `json:"repo"`
	WindowSeconds  int64                 `json:"window_seconds"`
	Closed         []standupClaimEvent   `json:"closed"`
	Reclaimed      []standupClaimEvent   `json:"reclaimed"`
	OpenClaim      *standupOpenClaim     `json:"open_claim,omitempty"`
	Stuck          []standupMessage      `json:"stuck"`
	UnansweredAsks []standupMessage      `json:"unanswered_asks"`
	ActiveTouches  []standupTouchSummary `json:"active_touches"`
}

type standupClaimEvent struct {
	ItemID    string `json:"item_id"`
	OutcomeAt int64  `json:"outcome_at"`
}

type standupOpenClaim struct {
	ItemID     string `json:"item_id"`
	ClaimedAt  int64  `json:"claimed_at"`
	LastTouch  int64  `json:"last_touch"`
	Intent     string `json:"intent,omitempty"`
	AgeSeconds int64  `json:"age_seconds"`
}

type standupMessage struct {
	ID     int64  `json:"id"`
	TS     int64  `json:"ts"`
	Thread string `json:"thread"`
	Body   string `json:"body"`
}

type standupTouchSummary struct {
	Path      string `json:"path"`
	ItemID    string `json:"item_id,omitempty"`
	StartedAt int64  `json:"started_at"`
}

func newStandupCmd() *cobra.Command {
	var (
		asJSON bool
		since  string
	)
	cmd := &cobra.Command{
		Use:   "standup",
		Short: "Per-agent digest: what closed, what's open, what's stuck, since the last 24h.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			window, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			report, err := buildStandup(ctx, bc, window)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderStandup(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "machine-readable output")
	cmd.Flags().StringVar(&since, "since", "24h", "lookback window (Go duration: 24h, 1h, 1w...)")
	return cmd
}

func buildStandup(ctx context.Context, bc *claimContext, window time.Duration) (*standupReport, error) {
	now := time.Now().Unix()
	cutoff := now - int64(window.Seconds())
	r := &standupReport{
		Agent:         bc.agentID,
		Repo:          bc.repoID,
		WindowSeconds: int64(window.Seconds()),
	}

	// Claims I closed in the window.
	closed, err := queryClaimEvents(ctx, bc.db, bc.repoID, bc.agentID, "done", cutoff)
	if err != nil {
		return nil, err
	}
	r.Closed = closed

	// Claims I lost (reclaimed by hygiene OR force-released by someone).
	reclaimed, err := queryClaimEventsAny(ctx, bc.db, bc.repoID, bc.agentID,
		[]string{"reclaimed", "force_released"}, cutoff)
	if err != nil {
		return nil, err
	}
	r.Reclaimed = reclaimed

	// Currently-open claim, if any.
	open, err := queryOpenClaim(ctx, bc.db, bc.repoID, bc.agentID, now)
	if err != nil {
		return nil, err
	}
	r.OpenClaim = open

	// Stuck messages I posted.
	stuck, err := queryMyMessages(ctx, bc.db, bc.repoID, bc.agentID, "stuck", cutoff)
	if err != nil {
		return nil, err
	}
	r.Stuck = stuck

	// Asks I posted that haven't been answered. Heuristic: my ask body
	// names a thread; we count it unanswered if there is no later
	// kind='answer' on the same thread by anyone in the window.
	asks, err := queryUnansweredAsks(ctx, bc.db, bc.repoID, bc.agentID, cutoff)
	if err != nil {
		return nil, err
	}
	r.UnansweredAsks = asks

	// Active touches I hold.
	touches, err := queryActiveTouches(ctx, bc.db, bc.repoID, bc.agentID)
	if err != nil {
		return nil, err
	}
	r.ActiveTouches = touches

	return r, nil
}

func queryClaimEvents(ctx context.Context, db *sql.DB, repoID, agentID, outcome string, cutoff int64) ([]standupClaimEvent, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT item_id, released_at FROM claim_history
		WHERE repo_id = ? AND agent_id = ? AND outcome = ? AND released_at >= ?
		ORDER BY released_at
	`, repoID, agentID, outcome, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []standupClaimEvent
	for rows.Next() {
		var e standupClaimEvent
		if err := rows.Scan(&e.ItemID, &e.OutcomeAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func queryClaimEventsAny(ctx context.Context, db *sql.DB, repoID, agentID string, outcomes []string, cutoff int64) ([]standupClaimEvent, error) {
	if len(outcomes) == 0 {
		return nil, nil
	}
	q := `SELECT item_id, released_at FROM claim_history WHERE repo_id = ? AND agent_id = ? AND released_at >= ? AND outcome IN (`
	args := []any{repoID, agentID, cutoff}
	for i, o := range outcomes {
		if i > 0 {
			q += ", "
		}
		q += "?"
		args = append(args, o)
	}
	q += ") ORDER BY released_at"
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []standupClaimEvent
	for rows.Next() {
		var e standupClaimEvent
		if err := rows.Scan(&e.ItemID, &e.OutcomeAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func queryOpenClaim(ctx context.Context, db *sql.DB, repoID, agentID string, now int64) (*standupOpenClaim, error) {
	var c standupOpenClaim
	var intent sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT item_id, claimed_at, last_touch, COALESCE(intent,'')
		FROM claims WHERE repo_id = ? AND agent_id = ? LIMIT 1
	`, repoID, agentID).Scan(&c.ItemID, &c.ClaimedAt, &c.LastTouch, &intent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Intent = intent.String
	c.AgeSeconds = now - c.ClaimedAt
	return &c, nil
}

func queryMyMessages(ctx context.Context, db *sql.DB, repoID, agentID, kind string, cutoff int64) ([]standupMessage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, ts, thread, COALESCE(body,'')
		FROM messages WHERE repo_id = ? AND agent_id = ? AND kind = ? AND ts >= ?
		ORDER BY ts
	`, repoID, agentID, kind, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []standupMessage
	for rows.Next() {
		var m standupMessage
		if err := rows.Scan(&m.ID, &m.TS, &m.Thread, &m.Body); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func queryUnansweredAsks(ctx context.Context, db *sql.DB, repoID, agentID string, cutoff int64) ([]standupMessage, error) {
	// Find every ask I posted in the window, then exclude those whose
	// thread saw an 'answer' kind from anyone afterwards.
	rows, err := db.QueryContext(ctx, `
		SELECT id, ts, thread, COALESCE(body,'')
		FROM messages WHERE repo_id = ? AND agent_id = ? AND kind = 'ask' AND ts >= ?
		ORDER BY ts
	`, repoID, agentID, cutoff)
	if err != nil {
		return nil, err
	}
	asks := []standupMessage{}
	for rows.Next() {
		var m standupMessage
		if err := rows.Scan(&m.ID, &m.TS, &m.Thread, &m.Body); err != nil {
			rows.Close()
			return nil, err
		}
		asks = append(asks, m)
	}
	rows.Close()
	if len(asks) == 0 {
		return nil, nil
	}
	out := make([]standupMessage, 0, len(asks))
	for _, a := range asks {
		var n int
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM messages
			WHERE repo_id = ? AND thread = ? AND kind = 'answer' AND ts > ?
		`, repoID, a.Thread, a.TS).Scan(&n); err != nil {
			return nil, err
		}
		if n == 0 {
			out = append(out, a)
		}
	}
	return out, nil
}

func queryActiveTouches(ctx context.Context, db *sql.DB, repoID, agentID string) ([]standupTouchSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT path, COALESCE(item_id,''), started_at FROM touches
		WHERE repo_id = ? AND agent_id = ? AND released_at IS NULL
		ORDER BY started_at
	`, repoID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []standupTouchSummary
	for rows.Next() {
		var t standupTouchSummary
		if err := rows.Scan(&t.Path, &t.ItemID, &t.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func renderStandup(w io.Writer, r *standupReport) {
	fmt.Fprintf(w, "standup for %s (last %s)\n", r.Agent, time.Duration(r.WindowSeconds)*time.Second)
	if len(r.Closed) > 0 {
		fmt.Fprintf(w, "\nshipped (%d):\n", len(r.Closed))
		for _, c := range r.Closed {
			fmt.Fprintf(w, "  - %s  done %s ago\n", c.ItemID, ago(c.OutcomeAt))
		}
	}
	if len(r.Reclaimed) > 0 {
		fmt.Fprintf(w, "\nlost (%d):\n", len(r.Reclaimed))
		for _, c := range r.Reclaimed {
			fmt.Fprintf(w, "  - %s  released %s ago\n", c.ItemID, ago(c.OutcomeAt))
		}
	}
	if r.OpenClaim != nil {
		fmt.Fprintf(w, "\nopen claim:\n  - %s  age %s",
			r.OpenClaim.ItemID, time.Duration(r.OpenClaim.AgeSeconds)*time.Second)
		if r.OpenClaim.Intent != "" {
			fmt.Fprintf(w, "  intent=%q", r.OpenClaim.Intent)
		}
		fmt.Fprintln(w)
	}
	if len(r.Stuck) > 0 {
		fmt.Fprintf(w, "\nstuck signals (%d):\n", len(r.Stuck))
		for _, m := range r.Stuck {
			fmt.Fprintf(w, "  - #%s  %s\n", m.Thread, trim(m.Body, 80))
		}
	}
	if len(r.UnansweredAsks) > 0 {
		fmt.Fprintf(w, "\nunanswered asks (%d):\n", len(r.UnansweredAsks))
		for _, m := range r.UnansweredAsks {
			fmt.Fprintf(w, "  - #%s  %s\n", m.Thread, trim(m.Body, 80))
		}
	}
	if len(r.ActiveTouches) > 0 {
		fmt.Fprintf(w, "\nactive touches (%d):\n", len(r.ActiveTouches))
		for _, t := range r.ActiveTouches {
			fmt.Fprintf(w, "  - %s\n", t.Path)
		}
	}
	if len(r.Closed) == 0 && len(r.Reclaimed) == 0 && r.OpenClaim == nil &&
		len(r.Stuck) == 0 && len(r.UnansweredAsks) == 0 && len(r.ActiveTouches) == 0 {
		fmt.Fprintln(w, "nothing to report.")
	}
}

func ago(ts int64) time.Duration {
	d := time.Since(time.Unix(ts, 0))
	if d < time.Second {
		return time.Second
	}
	return d.Truncate(time.Second)
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
