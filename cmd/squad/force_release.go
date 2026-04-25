package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

func newForceReleaseCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "force-release <ITEM-ID>",
		Short: "Admin: forcibly release someone else's stuck claim (requires --reason)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			prior, err := bc.store.ForceRelease(ctx, itemID, bc.agentID, reason)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "force-released %s (was held by %s)\n", itemID, prior)
				return nil
			}
			if errors.Is(err, claims.ErrReasonRequired) {
				fmt.Fprintln(cmd.ErrOrStderr(), "--reason is required for force-release")
				os.Exit(2)
			}
			if errors.Is(err, claims.ErrNotClaimed) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no active claim on %s\n", itemID)
				os.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why the claim is being forcibly released (required)")
	return cmd
}
