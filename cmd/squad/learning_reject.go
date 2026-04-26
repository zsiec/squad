package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

func newLearningRejectCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "reject <slug>",
		Short: "Archive a proposed learning under rejected/ (preserved for audit)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("--reason is required (rejected learnings are kept for audit)")
			}
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			l, err := learning.ResolveSingle(root, args[0])
			if err != nil {
				return err
			}
			if l.State != learning.StateProposed {
				return fmt.Errorf("learning %s is in state %s; only proposed can be rejected", l.Slug, l.State)
			}
			dst, err := learning.Promote(l, learning.StateRejected)
			if err != nil {
				return err
			}
			if err := appendRejectionReason(dst, reason); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rejected: %s\n", dst)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "rejection reason (required)")
	return cmd
}

func appendRejectionReason(path, reason string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	footer := fmt.Sprintf("\n\n## Rejection note (%s)\n\n%s\n",
		time.Now().UTC().Format("2006-01-02"), strings.TrimSpace(reason))
	return os.WriteFile(path, append(body, []byte(footer)...), 0o644)
}
