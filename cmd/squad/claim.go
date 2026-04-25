package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

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
				fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed\n", itemID)
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
