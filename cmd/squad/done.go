package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

func newDoneCmd() *cobra.Command {
	var summary string
	cmd := &cobra.Command{
		Use:   "done <ITEM-ID>",
		Short: "Mark an item done: release claim + rewrite frontmatter + move to .squad/done/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			itemPath := findItemPath(bc.itemsDir, itemID)
			err = bc.store.Done(ctx, itemID, bc.agentID, claims.DoneOpts{
				Summary:  summary,
				ItemPath: itemPath,
				DoneDir:  bc.doneDir,
			})
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "done %s\n", itemID)
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
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary appended to the done message")
	return cmd
}
