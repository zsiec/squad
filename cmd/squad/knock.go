package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newKnockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knock @<agent> <message>",
		Short: "High-priority directed message — interrupts the recipient's tick",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runKnockBody(ctx, bc.chat, bc.agentID, args); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	return cmd
}

func runKnockBody(ctx context.Context, c *chat.Chat, agentID string, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad knock @<agent> <message>")
		return 4
	}
	if !strings.HasPrefix(args[0], "@") {
		fmt.Fprintln(os.Stderr, "first argument must be @<agent>")
		return 4
	}
	target := strings.TrimPrefix(args[0], "@")
	body := strings.Join(args[1:], " ")
	if err := c.Knock(ctx, agentID, target, body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Printf("knocked @%s\n", target)
	return 0
}
