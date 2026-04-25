package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

type chattyVerb struct {
	kind  string
	short string
}

var (
	thinkingVerb  = chattyVerb{kind: chat.KindThinking, short: "Share where your head's at on the current claim"}
	stuckVerb     = chattyVerb{kind: chat.KindStuck, short: "Flag a blocker so peers can jump in"}
	milestoneVerb = chattyVerb{kind: chat.KindMilestone, short: "Mark a checkpoint (AC done, tests green, phase complete)"}
	fyiVerb       = chattyVerb{kind: chat.KindFYI, short: "Heads-up for the team"}
)

func newChattyCmd(verb chattyVerb) *cobra.Command {
	var (
		to      string
		mention string
	)
	cmd := &cobra.Command{
		Use:   verb.kind + " <message>",
		Short: verb.short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runChatty(ctx, bc.chat, bc.agentID, verb.kind, to, mention, args); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "thread (default: current claim, falls back to global)")
	cmd.Flags().StringVar(&mention, "mention", "", "comma-separated @ids")
	return cmd
}

func runChatty(ctx context.Context, c *chat.Chat, agentID, kind, to, mention string, args []string) int {
	body := strings.Join(args, " ")
	if strings.TrimSpace(body) == "" {
		fmt.Fprintf(os.Stderr, "usage: squad %s [--to ITEM|global] [--mention a,b] <message>\n", kind)
		return 4
	}
	thread := c.ResolveThread(ctx, agentID, to)
	mentions := splitMentions(mention)
	if mentions == nil {
		mentions = chat.ParseMentions(body)
	}
	if err := c.Post(ctx, chat.PostRequest{
		AgentID:  agentID,
		Thread:   thread,
		Kind:     kind,
		Body:     body,
		Mentions: mentions,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Printf("[%s -> #%s] %s\n", kind, thread, body)
	return 0
}
