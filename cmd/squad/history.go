package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <ITEM-ID>",
		Short: "Print all messages for an item, in time order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runHistoryBody(ctx, bc.chat, args[0], cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	return cmd
}

func runHistoryBody(ctx context.Context, c *chat.Chat, itemID string, w io.Writer) int {
	if itemID == "" {
		fmt.Fprintln(os.Stderr, "usage: squad history <ITEM-ID>")
		return 4
	}
	entries, err := c.History(ctx, itemID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Fprintf(w, "history for %s:\n", itemID)
	for _, e := range entries {
		fmt.Fprintf(w, "  #%-5d [%s] %s (%s): %s\n",
			e.ID, time.Unix(e.TS, 0).Format("2006-01-02 15:04"), e.Agent, e.Kind, e.Body)
	}
	return 0
}
