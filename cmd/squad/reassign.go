package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newReassignCmd() *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "reassign <ITEM-ID>",
		Short: "Transfer your claim by releasing it and @-mentioning the new owner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to <agent-id> is required")
			}
			target := strings.TrimPrefix(to, "@")
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			if err := bc.store.Reassign(ctx, itemID, bc.agentID, target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reassigned %s to %s\n", itemID, target)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "target agent id (with or without leading @)")
	return cmd
}
