package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

type TickArgs struct {
	Chat    *chat.Chat `json:"-"`
	AgentID string     `json:"agent_id"`
}

type TickResult struct {
	Digest chat.Digest `json:"digest"`
}

func Tick(ctx context.Context, args TickArgs) (*TickResult, error) {
	dg, err := args.Chat.Tick(ctx, args.AgentID)
	if err != nil {
		return nil, err
	}
	return &TickResult{Digest: dg}, nil
}

func newTickCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:    "tick",
		Short:  "Show new messages since last tick and advance the read cursor",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runTickBody(ctx, bc.chat, bc.agentID, jsonOut, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			maybePrintStaleChatNudge(ctx, bc.db, bc.repoID, bc.agentID, time.Now(), cmd.OutOrStdout())
			maybePrintTimeBoxNudge(ctx, bc.db, bc.repoID, bc.agentID, time.Now(), cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}

func runTickBody(ctx context.Context, c *chat.Chat, agentID string, jsonOut bool, w io.Writer) int {
	res, err := Tick(ctx, TickArgs{Chat: c, AgentID: agentID})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res.Digest)
		return 0
	}
	emitDigest(w, res.Digest)
	return 0
}

func emitDigest(w io.Writer, dg chat.Digest) {
	if len(dg.Knocks) == 0 && len(dg.Mentions) == 0 &&
		len(dg.Handoffs) == 0 && len(dg.Global) == 0 &&
		len(dg.YourThreads) == 0 && len(dg.LostClaims) == 0 {
		fmt.Fprintf(w, "no new messages for %s\n", dg.Agent)
		return
	}
	if len(dg.LostClaims) > 0 {
		fmt.Fprintf(w, "RECLAIMED while you were away: %v\n", dg.LostClaims)
	}
	digestSection(w, "KNOCKS (high priority)", dg.Knocks)
	digestSection(w, "MENTIONS", dg.Mentions)
	digestSection(w, "YOUR THREADS", dg.YourThreads)
	digestSection(w, "HANDOFFS", dg.Handoffs)
	digestSection(w, "GLOBAL", dg.Global)
}

func digestSection(w io.Writer, title string, msgs []chat.DigestMessage) {
	if len(msgs) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "== %s ==\n", title)
	for _, m := range msgs {
		fmt.Fprintf(w, "  [%s] %s (%s, #%s): %s\n",
			time.Unix(m.TS, 0).Format("15:04"), m.Agent, m.Kind, m.Thread, m.Body)
	}
}
