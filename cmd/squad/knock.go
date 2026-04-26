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

// KnockArgs is the input for Knock.
type KnockArgs struct {
	Chat    *chat.Chat
	AgentID string
	Target  string
	Body    string
}

// KnockResult reports a successful knock delivery.
type KnockResult struct {
	AgentID string `json:"agent_id"`
	Target  string `json:"target"`
}

// Knock sends a high-priority directed message that interrupts the
// recipient's tick.
func Knock(ctx context.Context, args KnockArgs) (*KnockResult, error) {
	target := strings.TrimPrefix(args.Target, "@")
	if target == "" {
		return nil, fmt.Errorf("knock: target agent required")
	}
	if args.Body == "" {
		return nil, fmt.Errorf("knock: message body required")
	}
	if err := args.Chat.Knock(ctx, args.AgentID, target, args.Body); err != nil {
		return nil, err
	}
	return &KnockResult{AgentID: args.AgentID, Target: target}, nil
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
	res, err := Knock(ctx, KnockArgs{
		Chat:    c,
		AgentID: agentID,
		Target:  args[0],
		Body:    strings.Join(args[1:], " "),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Printf("knocked @%s\n", res.Target)
	return 0
}
