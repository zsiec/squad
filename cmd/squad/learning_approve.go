package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

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
			l, err := learning.ResolveSingle(root, args[0])
			if err != nil {
				return err
			}
			if l.State != learning.StateProposed {
				return fmt.Errorf("learning %s is in state %s; only proposed can be approved", l.Slug, l.State)
			}
			dst, err := learning.Promote(l, learning.StateApproved)
			if err != nil {
				return err
			}
			if err := learning.RegenerateSkill(root); err != nil {
				return fmt.Errorf("skill regeneration: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "approved: %s\n", dst)
			return nil
		},
	}
}
