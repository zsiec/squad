package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

// ReassignArgs is the input for Reassign.
type ReassignArgs struct {
	Store   *claims.Store
	ItemID  string
	AgentID string
	Target  string
}

// ReassignResult reports a successful reassignment.
type ReassignResult struct {
	ItemID string `json:"item_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// Reassign transfers the caller's claim by releasing it and posting an
// @-mention to the new owner.
func Reassign(ctx context.Context, args ReassignArgs) (*ReassignResult, error) {
	target := strings.TrimPrefix(args.Target, "@")
	if target == "" {
		return nil, fmt.Errorf("reassign: target agent required")
	}
	if err := args.Store.Reassign(ctx, args.ItemID, args.AgentID, target); err != nil {
		return nil, err
	}
	return &ReassignResult{ItemID: args.ItemID, From: args.AgentID, To: target}, nil
}

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
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			res, err := Reassign(ctx, ReassignArgs{
				Store:   bc.store,
				ItemID:  itemID,
				AgentID: bc.agentID,
				Target:  to,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reassigned %s to %s\n", res.ItemID, res.To)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "target agent id (with or without leading @)")
	return cmd
}
