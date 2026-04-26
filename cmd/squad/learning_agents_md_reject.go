package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
	"github.com/zsiec/squad/internal/repo"
)

func newLearningAgentsMdRejectCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "agents-md-reject <id>",
		Short: "Archive a proposed AGENTS.md change under rejected/ (preserved for audit)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("--reason is required")
			}
			id := args[0]
			wd, _ := os.Getwd()
			root, err := repo.Discover(wd)
			if err != nil {
				return err
			}
			src := filepath.Join(root, ".squad", "learnings", "agents-md", "proposed", id+".md")
			body, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("read proposal: %w", err)
			}
			rewritten := learning.RewriteState(body, learning.State("rejected"))
			footer := fmt.Sprintf("\n\n## Rejection note (%s)\n\n%s\n",
				time.Now().UTC().Format("2006-01-02"), strings.TrimSpace(reason))
			rewritten = append(rewritten, []byte(footer)...)
			dst := filepath.Join(root, ".squad", "learnings", "agents-md", "rejected", id+".md")
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dst, rewritten, 0o644); err != nil {
				return err
			}
			if err := os.Remove(src); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rejected: %s\n", dst)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "rejection reason (required)")
	return cmd
}
