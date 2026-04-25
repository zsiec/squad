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

func newTickCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "tick",
		Short: "Show new messages since last tick and advance the read cursor",
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
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "machine-readable output")
	return cmd
}

func runTickBody(ctx context.Context, c *chat.Chat, agentID string, jsonOut bool, w io.Writer) int {
	dg, err := c.Tick(ctx, agentID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(dg)
		return 0
	}
	emitDigest(w, dg)
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
