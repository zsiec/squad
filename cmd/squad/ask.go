package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newAskCmd() *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "ask @<agent> <message>",
		Short: "Ask a question of a specific agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runAskBody(ctx, bc.chat, bc.agentID, to, args); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "global", "thread")
	return cmd
}

func runAskBody(ctx context.Context, c *chat.Chat, agentID, to string, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad ask @<agent> [--to ITEM|global] <message>")
		return 4
	}
	if !strings.HasPrefix(args[0], "@") {
		fmt.Fprintln(os.Stderr, "first argument must be @<agent>")
		return 4
	}
	target := strings.TrimPrefix(args[0], "@")
	body := strings.Join(args[1:], " ")
	if to == "" {
		to = chat.ThreadGlobal
	}
	if err := c.Ask(ctx, agentID, to, target, body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}
