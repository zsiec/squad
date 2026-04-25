package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

type mailboxArgs struct {
	Format string
	Event  string
}

func newMailboxCmd() *cobra.Command {
	var a mailboxArgs
	cmd := &cobra.Command{
		Use:   "mailbox",
		Short: "Print pending mailbox as a Claude Code hook envelope; exit 0 if empty",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			os.Exit(runMailbox(ctx, bc.chat, bc.agentID, a, cmd.OutOrStdout()))
			return nil
		},
	}
	cmd.Flags().StringVar(&a.Format, "format", "additional-context",
		"output envelope: 'additional-context' (UserPromptSubmit/PostToolUse) or 'decision-block' (Stop)")
	cmd.Flags().StringVar(&a.Event, "event", "UserPromptSubmit",
		"hookEventName when --format=additional-context")
	return cmd
}

func runMailbox(ctx context.Context, c *chat.Chat, agentID string, a mailboxArgs, stdout io.Writer) int {
	m, err := c.Mailbox(ctx, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "squad mailbox: %v\n", err)
		return 4
	}
	if m.Empty() {
		return 0
	}
	switch a.Format {
	case "decision-block":
		fmt.Fprintln(stdout, m.FormatJSON())
		_ = c.MarkMailboxRead(ctx, agentID, m)
		return 2
	case "additional-context":
		out, err := json.Marshal(struct {
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}{
			HookSpecificOutput: struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			}{
				HookEventName:     a.Event,
				AdditionalContext: m.Format(),
			},
		})
		if err != nil {
			return 4
		}
		fmt.Fprintln(stdout, string(out))
		_ = c.MarkMailboxRead(ctx, agentID, m)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "squad mailbox: unknown --format=%q\n", a.Format)
		return 4
	}
}
