package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newAnswerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "answer <msg-id> <message>",
		Short: "Answer a previous message by id",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runAnswerBody(ctx, bc.chat, bc.agentID, args); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	return cmd
}

func runAnswerBody(ctx context.Context, c *chat.Chat, agentID string, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad answer <msg-id> <message>")
		return 4
	}
	ref, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || ref <= 0 {
		fmt.Fprintf(os.Stderr, "invalid msg-id: %q\n", args[0])
		return 4
	}
	body := strings.Join(args[1:], " ")
	if err := c.Answer(ctx, agentID, ref, body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}
