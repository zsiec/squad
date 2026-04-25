package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

func lookupClaimHolder(ctx context.Context, bc *claimContext, itemID string) string {
	var agent string
	err := bc.db.QueryRowContext(ctx,
		`SELECT agent_id FROM claims WHERE item_id = ? AND repo_id = ?`,
		itemID, bc.repoID).Scan(&agent)
	if err != nil {
		return ""
	}
	return agent
}

func newClaimCmd() *cobra.Command {
	var (
		intent  string
		touches string
		long    bool
	)
	cmd := &cobra.Command{
		Use:   "claim <ITEM-ID>",
		Short: "Atomically claim an item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			var touchList []string
			if touches != "" {
				for _, p := range strings.Split(touches, ",") {
					if p = strings.TrimSpace(p); p != "" {
						touchList = append(touchList, p)
					}
				}
			}

			err = bc.store.Claim(ctx, itemID, bc.agentID, intent, touchList, long,
				claims.ClaimWithPreflight(bc.itemsDir, bc.doneDir))
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "claimed %s\n", itemID)
				return nil
			}
			if errors.Is(err, claims.ErrClaimTaken) {
				holder := lookupClaimHolder(ctx, bc, itemID)
				if holder != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed by %s\n", itemID, holder)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed\n", itemID)
				}
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrBlockedByOpen) {
				fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
				os.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&intent, "intent", "", "short sentence describing intent")
	cmd.Flags().StringVar(&touches, "touches", "", "comma-separated file paths you'll modify")
	cmd.Flags().BoolVar(&long, "long", false, "extended stale threshold (2h instead of 30m)")
	return cmd
}
