package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

func newReleaseCmd() *cobra.Command {
	var outcome string
	cmd := &cobra.Command{
		Use:   "release <ITEM-ID>",
		Short: "Release your claim on an item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			err = bc.store.Release(ctx, itemID, bc.agentID, outcome)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "released %s (%s)\n", itemID, outcome)
				return nil
			}
			if errors.Is(err, claims.ErrNotYours) {
				fmt.Fprintln(cmd.ErrOrStderr(), "not your claim")
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrNotClaimed) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no active claim on %s\n", itemID)
				os.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&outcome, "outcome", "released", "outcome string recorded in claim_history")
	return cmd
}
