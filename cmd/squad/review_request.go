package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newReviewRequestCmd() *cobra.Command {
	var mention string
	cmd := &cobra.Command{
		Use:   "review-request <ITEM-ID>",
		Short: "Request review on an item, optionally mentioning a reviewer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runReviewRequestBody(ctx, bc.chat, bc.agentID, args[0], mention); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mention, "mention", "", "reviewer agent id (with or without @)")
	return cmd
}

func runReviewRequestBody(ctx context.Context, c *chat.Chat, agentID, itemID, mention string) int {
	if itemID == "" {
		fmt.Fprintln(os.Stderr, "usage: squad review-request <ITEM-ID> [--mention @<agent>]")
		return 4
	}
	reviewer := strings.TrimPrefix(mention, "@")
	if err := c.ReviewRequest(ctx, agentID, itemID, reviewer); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}
