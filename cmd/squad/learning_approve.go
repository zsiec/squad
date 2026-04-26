package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

type LearningApproveArgs struct {
	RepoRoot string `json:"repo_root"`
	Slug     string `json:"slug"`
}

type LearningApproveResult struct {
	Path     string             `json:"path"`
	Learning *learning.Learning `json:"learning"`
}

func LearningApprove(_ context.Context, args LearningApproveArgs) (*LearningApproveResult, error) {
	l, err := resolveLearning(args.RepoRoot, args.Slug)
	if err != nil {
		return nil, err
	}
	if l.State != learning.StateProposed {
		return nil, fmt.Errorf("%w: %s is in state %s", ErrNotProposed, l.Slug, l.State)
	}
	dst, err := learning.Promote(l, learning.StateApproved, nil)
	if err != nil {
		return nil, err
	}
	if err := learning.RegenerateSkill(args.RepoRoot); err != nil {
		return nil, fmt.Errorf("skill regeneration: %w", err)
	}
	parsed, perr := learning.Parse(dst)
	if perr != nil {
		return nil, perr
	}
	return &LearningApproveResult{Path: dst, Learning: &parsed}, nil
}

func newLearningApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <slug>",
		Short: "Promote a proposed learning to approved/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			res, err := LearningApprove(cmd.Context(), LearningApproveArgs{
				RepoRoot: root, Slug: args[0],
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "approved: %s\n", res.Path)
			return nil
		},
	}
}

func resolveLearning(repoRoot, slug string) (learning.Learning, error) {
	return learning.ResolveSingle(repoRoot, slug)
}
