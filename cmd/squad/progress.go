package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

// ErrNotYours / ErrNotClaimed re-export claims package sentinels so the
// pure-function callers (CLI + MCP) can errors.Is without depending on
// internal/claims directly.
var (
	ErrNotYours   = claims.ErrNotYours
	ErrNotClaimed = claims.ErrNotClaimed
)

type ProgressArgs struct {
	DB      *sql.DB    `json:"-"`
	RepoID  string     `json:"repo_id"`
	Chat    *chat.Chat `json:"-"`
	AgentID string     `json:"agent_id"`
	ItemID  string     `json:"item_id"`
	Pct     int        `json:"pct"`
	Note    string     `json:"note,omitempty"`
}

type ProgressResult struct {
	ItemID   string `json:"item_id"`
	Pct      int    `json:"pct"`
	Note     string `json:"note,omitempty"`
	PostedAt int64  `json:"posted_at"`
}

func Progress(ctx context.Context, args ProgressArgs) (*ProgressResult, error) {
	var holder string
	row := args.DB.QueryRowContext(ctx,
		`SELECT agent_id FROM claims WHERE item_id = ? AND repo_id = ?`, args.ItemID, args.RepoID)
	switch err := row.Scan(&holder); err {
	case nil:
		if holder != args.AgentID {
			return nil, fmt.Errorf("%w: %s is held by %s", ErrNotYours, args.ItemID, holder)
		}
	case sql.ErrNoRows:
		return nil, fmt.Errorf("%w: %s", ErrNotClaimed, args.ItemID)
	default:
		return nil, err
	}

	if err := args.Chat.ReportProgress(ctx, args.AgentID, args.ItemID, args.Pct, args.Note); err != nil {
		return nil, err
	}
	// PostProgress is best-effort; the underlying notify is async, so any
	// error here is swallowed and PostedAt always records when Progress ran.
	posted := time.Now().Unix()
	_ = args.Chat.PostProgress(ctx, args.AgentID, args.ItemID, args.Pct, args.Note)
	return &ProgressResult{ItemID: args.ItemID, Pct: args.Pct, Note: args.Note, PostedAt: posted}, nil
}

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

	res, err := Progress(ctx, ProgressArgs{
		DB:      db,
		RepoID:  repoID,
		Chat:    c,
		AgentID: agentID,
		ItemID:  itemID,
		Pct:     pct,
		Note:    note,
	})
	switch {
	case errors.Is(err, ErrNotYours):
		fmt.Fprintln(os.Stderr, err)
		return 1
	case errors.Is(err, ErrNotClaimed):
		fmt.Fprintf(os.Stderr, "%s is not claimed; claim it first with `squad claim %s`\n", itemID, itemID)
		return 1
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		return 4
	}

	suffix := ""
	if res.Note != "" {
		suffix = ": " + res.Note
	}
	fmt.Fprintf(stdout, "[progress -> #%s] %d%%%s\n", res.ItemID, res.Pct, suffix)
	return 0
}
