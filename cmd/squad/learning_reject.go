package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

type LearningRejectArgs struct {
	RepoRoot string `json:"repo_root"`
	Slug     string `json:"slug"`
	Reason   string `json:"reason"`

	Now func() time.Time `json:"-"`
}

type LearningRejectResult struct {
	Path     string             `json:"path"`
	Learning *learning.Learning `json:"learning"`
}

func LearningReject(_ context.Context, args LearningRejectArgs) (*LearningRejectResult, error) {
	if strings.TrimSpace(args.Reason) == "" {
		return nil, ErrReasonRequired
	}
	clock := args.Now
	if clock == nil {
		clock = time.Now
	}
	l, err := resolveLearning(args.RepoRoot, args.Slug)
	if err != nil {
		return nil, err
	}
	if l.State != learning.StateProposed {
		return nil, fmt.Errorf("%w: %s is in state %s", ErrNotProposed, l.Slug, l.State)
	}
	dst, err := learning.Promote(l, learning.StateRejected)
	if err != nil {
		return nil, err
	}
	if err := appendRejectionReason(dst, args.Reason, clock()); err != nil {
		return nil, err
	}
	parsed, perr := learning.Parse(dst)
	if perr != nil {
		return nil, perr
	}
	return &LearningRejectResult{Path: dst, Learning: &parsed}, nil
}

func newLearningRejectCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "reject <slug>",
		Short: "Archive a proposed learning under rejected/ (preserved for audit)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			res, err := LearningReject(cmd.Context(), LearningRejectArgs{
				RepoRoot: root, Slug: args[0], Reason: reason,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rejected: %s\n", res.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "rejection reason (required)")
	return cmd
}

func appendRejectionReason(path, reason string, now time.Time) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	footer := fmt.Sprintf("\n\n## Rejection note (%s)\n\n%s\n",
		now.UTC().Format("2006-01-02"), strings.TrimSpace(reason))
	return os.WriteFile(path, append(body, []byte(footer)...), 0o644)
}
