package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

// SayArgs is the pure-function input. The cobra wrapper resolves the
// "empty default → current claim or global" routing before calling Say;
// To here must already be a fully-resolved target like "global" or
// "<item-id>". Verb defaults to chat.KindSay when empty.
type SayArgs struct {
	Chat     *chat.Chat `json:"-"`
	AgentID  string     `json:"agent_id"`
	To       string     `json:"to"`
	Body     string     `json:"body"`
	Mentions []string   `json:"mentions,omitempty"`
	Verb     string     `json:"verb,omitempty"`
}

type SayResult struct {
	To       string   `json:"to"`
	Body     string   `json:"body"`
	Mentions []string `json:"mentions,omitempty"`
	Verb     string   `json:"verb"`
	PostedAt int64    `json:"posted_at"`
}

func Say(ctx context.Context, args SayArgs) (*SayResult, error) {
	if strings.TrimSpace(args.Body) == "" {
		return nil, fmt.Errorf("say: body required")
	}
	verb := args.Verb
	if verb == "" {
		verb = chat.KindSay
	}
	to := args.To
	if to == "" {
		to = chat.ThreadGlobal
	}
	if err := args.Chat.Post(ctx, chat.PostRequest{
		AgentID:  args.AgentID,
		Thread:   to,
		Kind:     verb,
		Body:     args.Body,
		Mentions: args.Mentions,
		Priority: chat.PriorityNormal,
	}); err != nil {
		return nil, err
	}
	return &SayResult{
		To:       to,
		Body:     args.Body,
		Mentions: args.Mentions,
		Verb:     verb,
		PostedAt: time.Now().Unix(),
	}, nil
}

type sayCmdFlags struct {
	To      string
	Mention string
	Body    string
}

func newSayCmd() *cobra.Command {
	var a sayCmdFlags
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

func runSayBody(ctx context.Context, c *chat.Chat, agentID string, a sayCmdFlags, stdout io.Writer) int {
	if strings.TrimSpace(a.Body) == "" {
		fmt.Fprintln(os.Stderr, "usage: squad say [--to ITEM|global] [--mention a,b] <message>")
		return 4
	}
	thread, err := c.ResolveThread(ctx, agentID, a.To)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	res, err := Say(ctx, SayArgs{
		Chat:     c,
		AgentID:  agentID,
		To:       thread,
		Body:     a.Body,
		Mentions: splitMentions(a.Mention),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	label := res.To
	if label == chat.ThreadGlobal {
		label = "global"
	}
	fmt.Fprintf(stdout, "[say -> #%s] %s\n", label, res.Body)
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
