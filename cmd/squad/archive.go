package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/store"
)

func newArchiveCmd() *cobra.Command {
	var before string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Roll old chat messages into a per-month archive DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			dur, err := parseHumanDuration(before)
			if err != nil {
				return fmt.Errorf("invalid --before: %w", err)
			}
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			home, err := store.Home()
			if err != nil {
				return err
			}
			dir := filepath.Join(home, "archive")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			cutoff := time.Now().Add(-dur).Unix()
			n, path, err := hygiene.Archive(ctx, bc.db, bc.repoID, dir, cutoff)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "archived %d message(s) to %s\n", n, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&before, "before", "30d", "cut-off duration (e.g. 30d, 720h)")
	return cmd
}

func parseHumanDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	var days int
	if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("could not parse %q", s)
}
