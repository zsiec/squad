package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newWhoCmd() *cobra.Command {
	var (
		jsonOut    bool
		activeOnly bool
	)
	cmd := &cobra.Command{
		Use:   "who",
		Short: "List registered agents with status, current claim, and last tick",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runWhoBody(ctx, bc.chat, jsonOut, activeOnly, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&activeOnly, "active-only", false, "hide stale/offline agents")
	return cmd
}

func runWhoBody(ctx context.Context, c *chat.Chat, jsonOut, activeOnly bool, w io.Writer) int {
	rows, err := c.WhoList(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if activeOnly {
		filtered := rows[:0]
		for _, r := range rows {
			if r.Status == "active" || r.Status == "idle" {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rows)
		return 0
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "AGENT\tNAME\tCLAIM\tINTENT\tFILES\tTICK\tSTATUS")
	for _, r := range rows {
		tick := "-"
		if r.LastTick > 0 {
			tick = time.Unix(r.LastTick, 0).Format("15:04")
		}
		intent := r.Intent
		if len(intent) > 40 {
			intent = intent[:40]
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			r.AgentID, r.DisplayName, r.ClaimItem, intent, r.TouchCount, tick, r.Status)
	}
	return 0
}
