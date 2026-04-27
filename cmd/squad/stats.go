package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/stats"
)

type StatsArgs struct {
	DB     *sql.DB       `json:"-"`
	RepoID string        `json:"repo_id"`
	Window time.Duration `json:"window,omitempty"`
}

// StatsResult is the structured operational snapshot. Aliasing
// stats.Snapshot keeps the wire shape identical to the Compute output —
// MCP callers see the same JSON the CLI's --json flag emits.
type StatsResult = stats.Snapshot

func Stats(ctx context.Context, args StatsArgs) (*StatsResult, error) {
	snap, err := stats.Compute(ctx, args.DB, stats.ComputeOpts{RepoID: args.RepoID, Window: args.Window})
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

func newStatsCmd() *cobra.Command {
	var jsonOut, tail bool
	var window, interval time.Duration
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Operational statistics: verification rate, claim p99, WIP violations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if !tail {
				snap, err := Stats(ctx, StatsArgs{DB: bc.db, RepoID: bc.repoID, Window: window})
				if err != nil {
					return err
				}
				if jsonOut {
					return writeIndented(cmd.OutOrStdout(), snap)
				}
				renderHumanStats(cmd.OutOrStdout(), *snap)
				return nil
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			enc := json.NewEncoder(cmd.OutOrStdout())
			for {
				snap, err := Stats(ctx, StatsArgs{DB: bc.db, RepoID: bc.repoID, Window: window})
				if err != nil {
					return err
				}
				if err := enc.Encode(snap); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit pretty-printed JSON snapshot")
	cmd.Flags().BoolVar(&tail, "tail", false, "stream NDJSON until interrupted")
	cmd.Flags().DurationVar(&window, "window", 24*time.Hour, "metric window (0 = unbounded)")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "tail emit interval")
	return cmd
}

func writeIndented(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// renderHumanStats prints a one-line-per-metric summary; only fields with
// data are emitted so an empty repo doesn't show "0% verification rate".
func renderHumanStats(w io.Writer, s stats.Snapshot) {
	fmt.Fprintf(w, "items: total=%d open=%d claimed=%d blocked=%d done=%d\n",
		s.Items.Total, s.Items.Open, s.Items.Claimed, s.Items.Blocked, s.Items.Done)
	fmt.Fprintf(w, "claims: active=%d completed=%d wip_violations=%d\n",
		s.Claims.Active, s.Claims.CompletedInWindow, s.Claims.WIPViolationsAttempted)
	if p := s.Claims.DurationSeconds; p.Count > 0 {
		fmt.Fprintf(w, "claim duration: p50=%s p90=%s p99=%s n=%d\n",
			fmtDur(p.P50), fmtDur(p.P90), fmtDur(p.P99), p.Count)
	}
	if r := s.Verification.Rate; r != nil {
		fmt.Fprintf(w, "verification rate: %.1f%% (%d/%d dones)\n", *r*100,
			s.Verification.DonesWithFullEvidence, s.Verification.DonesTotal)
	}
	if r := s.Verification.ReviewerDisagreementRate; r != nil {
		fmt.Fprintf(w, "reviewer disagreement: %.1f%% (%d/%d)\n", *r*100,
			s.Verification.ReviewsWithDisagreement, s.Verification.ReviewsTotal)
	}
	if r := s.Learnings.RepeatMistakeRate; r != nil {
		fmt.Fprintf(w, "repeat-mistake rate: %.1f%% (%d/%d new bugs)\n", *r*100,
			s.Learnings.RepeatMistakesInWindow, s.Learnings.NewBugsInWindow)
	}
}

func fmtDur(p *float64) string {
	if p == nil {
		return "-"
	}
	return time.Duration(*p * float64(time.Second)).Truncate(time.Second).String()
}
