package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

func newBlockedCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "blocked <ITEM-ID>",
		Short: "Mark item blocked: release claim + status: blocked + ensure ## Blocker section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			if reason == "" {
				return fmt.Errorf("--reason is required for blocked")
			}
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			itemPath := findItemPath(bc.itemsDir, itemID)
			err = bc.store.Blocked(ctx, itemID, bc.agentID, claims.BlockedOpts{
				Reason:   reason,
				ItemPath: itemPath,
			})
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "blocked %s\n", itemID)
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
	cmd.Flags().StringVar(&reason, "reason", "", "blocker description (required)")
	return cmd
}
