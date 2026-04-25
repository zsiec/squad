package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/notify"
)

func newNotifyCleanupCmd() *cobra.Command {
	var instance string
	cmd := &cobra.Command{
		Use:    "notify-cleanup",
		Short:  "Drop notify_endpoints rows for an instance (called by SessionEnd hook)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			r := notify.NewRegistry(bc.db)
			os.Exit(runNotifyCleanup(ctx, r, resolveInstance(instance)))
			return nil
		},
	}
	cmd.Flags().StringVar(&instance, "instance", "", "instance id (default: env-derived)")
	return cmd
}

func runNotifyCleanup(ctx context.Context, r *notify.Registry, instance string) int {
	if instance == "" {
		fmt.Fprintln(os.Stderr, "squad notify-cleanup: instance required")
		return 4
	}
	if err := r.UnregisterInstance(ctx, instance); err != nil {
		fmt.Fprintf(os.Stderr, "squad notify-cleanup: %v\n", err)
		return 4
	}
	return 0
}
