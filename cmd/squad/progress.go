package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newProgressCmd() *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "progress <ITEM-ID> <pct 0..100>",
		Short: "Report progress on a held item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runProgressBody(ctx, bc.db, bc.repoID, bc.chat, bc.agentID, args[0], args[1], note, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "optional note")
	return cmd
}

func runProgressBody(ctx context.Context, db *sql.DB, repoID string, c *chat.Chat, agentID, itemID, pctStr, note string, stdout io.Writer) int {
	if itemID == "" {
		fmt.Fprintln(os.Stderr, "usage: squad progress <ITEM-ID> <pct 0..100> [--note ...]")
		return 4
	}
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid pct: %v\n", err)
		return 4
	}

	var holder string
	row := db.QueryRowContext(ctx,
		`SELECT agent_id FROM claims WHERE item_id = ? AND repo_id = ?`, itemID, repoID)
	switch err := row.Scan(&holder); err {
	case nil:
		if holder != agentID {
			fmt.Fprintf(os.Stderr, "%s is held by %s; only the holder can report progress\n", itemID, holder)
			return 1
		}
	case sql.ErrNoRows:
		fmt.Fprintf(os.Stderr, "%s is not claimed; claim it first with `squad claim %s`\n", itemID, itemID)
		return 1
	default:
		fmt.Fprintln(os.Stderr, err)
		return 4
	}

	if err := c.ReportProgress(ctx, agentID, itemID, pct, note); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if err := c.PostProgress(ctx, agentID, itemID, pct, note); err != nil {
		// Non-fatal: the progress row is durable; the chat post is a courtesy.
		fmt.Fprintf(os.Stderr, "warning: progress chat post failed: %v\n", err)
	}
	suffix := ""
	if note != "" {
		suffix = ": " + note
	}
	fmt.Fprintf(stdout, "[progress -> #%s] %d%%%s\n", itemID, pct, suffix)
	return 0
}
