package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

type sayArgs struct {
	To      string
	Mention string
	Body    string
}

func newSayCmd() *cobra.Command {
	var a sayArgs
	cmd := &cobra.Command{
		Use:   "say <message>",
		Short: "Post a message to the current claim's thread or global",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.Body = strings.Join(args, " ")
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runSayBody(ctx, bc.chat, bc.agentID, a, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&a.To, "to", "", "thread: 'global' or '<ITEM-ID>' (default: current claim, fallback to global)")
	cmd.Flags().StringVar(&a.Mention, "mention", "", "comma-separated @ids overriding inline parsing")
	return cmd
}

func runSayBody(ctx context.Context, c *chat.Chat, agentID string, a sayArgs, stdout io.Writer) int {
	if strings.TrimSpace(a.Body) == "" {
		fmt.Fprintln(os.Stderr, "usage: squad say [--to ITEM|global] [--mention a,b] <message>")
		return 4
	}
	mentions := splitMentions(a.Mention)
	thread := c.ResolveThread(ctx, agentID, a.To)
	if err := c.Post(ctx, chat.PostRequest{
		AgentID:  agentID,
		Thread:   thread,
		Kind:     chat.KindSay,
		Body:     a.Body,
		Mentions: mentions,
		Priority: chat.PriorityNormal,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	label := thread
	if label == chat.ThreadGlobal {
		label = "global"
	}
	fmt.Fprintf(stdout, "[say -> #%s] %s\n", label, a.Body)
	return 0
}

func splitMentions(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(m)
		m = strings.TrimPrefix(m, "@")
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}
