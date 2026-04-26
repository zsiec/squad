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

// HistoryArgs is the input for History.
type HistoryArgs struct {
	Chat   *chat.Chat
	ItemID string
}

// HistoryEntry is one row of an item's chat history. TS is unix seconds so
// JSON consumers can format on their side.
type HistoryEntry struct {
	ID    int64  `json:"id"`
	TS    int64  `json:"ts"`
	Agent string `json:"agent"`
	Kind  string `json:"kind"`
	Body  string `json:"body"`
}

// HistoryResult is the structured form of one item's full chat history.
type HistoryResult struct {
	ItemID  string         `json:"item_id"`
	Entries []HistoryEntry `json:"entries"`
}

// History returns every chat message attached to an item, in time order.
func History(ctx context.Context, args HistoryArgs) (*HistoryResult, error) {
	if args.ItemID == "" {
		return nil, fmt.Errorf("history: item id required")
	}
	entries, err := args.Chat.History(ctx, args.ItemID)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, HistoryEntry{
			ID: e.ID, TS: e.TS, Agent: e.Agent, Kind: e.Kind, Body: e.Body,
		})
	}
	return &HistoryResult{ItemID: args.ItemID, Entries: out}, nil
}

func runHistoryBody(ctx context.Context, c *chat.Chat, itemID string, w io.Writer) int {
	res, err := History(ctx, HistoryArgs{Chat: c, ItemID: itemID})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Fprintf(w, "history for %s:\n", res.ItemID)
	for _, e := range res.Entries {
		fmt.Fprintf(w, "  #%-5d [%s] %s (%s): %s\n",
			e.ID, time.Unix(e.TS, 0).Format("2006-01-02 15:04"), e.Agent, e.Kind, e.Body)
	}
	return 0
}
