package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

type LearningAgentsMdRejectArgs struct {
	RepoRoot string `json:"repo_root"`
	ID       string `json:"id"`
	Reason   string `json:"reason"`

	Now func() time.Time `json:"-"`
}

type LearningAgentsMdRejectResult struct {
	Path string `json:"path"`
}

func LearningAgentsMdReject(_ context.Context, args LearningAgentsMdRejectArgs) (*LearningAgentsMdRejectResult, error) {
	if strings.TrimSpace(args.Reason) == "" {
		return nil, ErrReasonRequired
	}
	clock := args.Now
	if clock == nil {
		clock = time.Now
	}
	src := filepath.Join(args.RepoRoot, ".squad", "learnings", "agents-md", "proposed", args.ID+".md")
	body, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrProposalNotFound, src)
		}
		return nil, fmt.Errorf("read proposal: %w", err)
	}
	rewritten := learning.RewriteState(body, learning.State("rejected"))
	footer := fmt.Sprintf("\n\n## Rejection note (%s)\n\n%s\n",
		clock().UTC().Format("2006-01-02"), strings.TrimSpace(args.Reason))
	rewritten = append(rewritten, []byte(footer)...)
	dst := filepath.Join(args.RepoRoot, ".squad", "learnings", "agents-md", "rejected", args.ID+".md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(dst, rewritten, 0o644); err != nil {
		return nil, err
	}
	if err := os.Remove(src); err != nil {
		return nil, err
	}
	return &LearningAgentsMdRejectResult{Path: dst}, nil
}

func newLearningAgentsMdRejectCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "agents-md-reject <id>",
		Short: "Archive a proposed AGENTS.md change under rejected/ (preserved for audit)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			res, err := LearningAgentsMdReject(cmd.Context(), LearningAgentsMdRejectArgs{
				RepoRoot: root, ID: args[0], Reason: reason,
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
