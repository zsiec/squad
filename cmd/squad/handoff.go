package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

func newHandoffCmd() *cobra.Command {
	var (
		shipped   []string
		inflight  []string
		surprised []string
		unblocks  []string
		note      string
		stdinIn   bool
	)
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Post a handoff brief and release any held claims",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			h := chat.HandoffBody{
				Shipped:     shipped,
				InFlight:    inflight,
				SurprisedBy: surprised,
				Unblocks:    unblocks,
				Note:        note,
			}
			if stdinIn {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				extra := strings.TrimSpace(string(data))
				switch {
				case h.Note != "" && extra != "":
					h.Note = h.Note + "\n\n" + extra
				case extra != "":
					h.Note = extra
				}
			}
			if code := runHandoffBody(ctx, bc.chat, bc.store, bc.agentID, h); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&shipped, "shipped", nil, "item shipped this session (repeatable)")
	cmd.Flags().StringSliceVar(&inflight, "in-flight", nil, "item still in flight (repeatable)")
	cmd.Flags().StringSliceVar(&surprised, "surprised-by", nil, "surprising finding (repeatable)")
	cmd.Flags().StringSliceVar(&unblocks, "unblocks", nil, "item unblocked by this work (repeatable)")
	cmd.Flags().StringVar(&note, "note", "", "free-form note")
	cmd.Flags().BoolVar(&stdinIn, "stdin", false, "append note body from stdin")
	return cmd
}

func runHandoffBody(ctx context.Context, c *chat.Chat, claimStore *claims.Store, agentID string, h chat.HandoffBody) int {
	if h.Empty() {
		fmt.Fprintln(os.Stderr, "handoff requires at least one --shipped / --in-flight / --surprised-by / --unblocks / --note (or --stdin)")
		return 4
	}
	if err := c.PostHandoff(ctx, agentID, h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if err := claimStore.ReleaseAll(ctx, agentID, "handoff"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Printf("handoff posted by %s (%s)\n", agentID, h.Summary())
	return 0
}
