package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

var ErrItemRequired = errors.New("review-request: item id required")

type ReviewRequestArgs struct {
	Chat     *chat.Chat `json:"-"`
	AgentID  string     `json:"agent_id"`
	ItemID   string     `json:"item_id"`
	Reviewer string     `json:"reviewer,omitempty"`
}

type ReviewRequestResult struct {
	ItemID   string `json:"item_id"`
	Reviewer string `json:"reviewer,omitempty"`
}

func ReviewRequest(ctx context.Context, args ReviewRequestArgs) (*ReviewRequestResult, error) {
	if args.ItemID == "" {
		return nil, ErrItemRequired
	}
	reviewer := strings.TrimPrefix(args.Reviewer, "@")
	if err := args.Chat.ReviewRequest(ctx, args.AgentID, args.ItemID, reviewer); err != nil {
		return nil, err
	}
	return &ReviewRequestResult{ItemID: args.ItemID, Reviewer: reviewer}, nil
}

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
			if code := runReviewRequestBody(ctx, bc.chat, bc.agentID, args[0], mention, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mention, "mention", "", "reviewer agent id (with or without @)")
	return cmd
}

func runReviewRequestBody(ctx context.Context, c *chat.Chat, agentID, itemID, mention string, stdout io.Writer) int {
	res, err := ReviewRequest(ctx, ReviewRequestArgs{
		Chat:     c,
		AgentID:  agentID,
		ItemID:   itemID,
		Reviewer: mention,
	})
	if errors.Is(err, ErrItemRequired) {
		fmt.Fprintln(os.Stderr, "usage: squad review-request <ITEM-ID> [--mention @<agent>]")
		return 4
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	suffix := ""
	if res.Reviewer != "" {
		suffix = " (cc @" + res.Reviewer + ")"
	}
	fmt.Fprintf(stdout, "[review-request -> #%s]%s\n", res.ItemID, suffix)
	return 0
}
