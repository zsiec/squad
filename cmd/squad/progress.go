package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newProgressCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "progress <ITEM-ID> <pct 0..100>",
		Short: "Report progress on a held item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runProgressBody(ctx, bc.chat, bc.agentID, args[0], args[1], note); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "optional note")
	return cmd
}

func runProgressBody(ctx context.Context, c *chat.Chat, agentID, itemID, pctStr, note string) int {
	if itemID == "" {
		fmt.Fprintln(os.Stderr, "usage: squad progress <ITEM-ID> <pct 0..100> [--note ...]")
		return 4
	}
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid pct: %v\n", err)
		return 4
	}
	if err := c.ReportProgress(ctx, agentID, itemID, pct, note); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}
