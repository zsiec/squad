package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
)

// ErrBlockedReasonRequired is the typed error for the boundary check that
// "blocked" requires a non-empty reason. The cobra wrapper renders the
// existing CLI message when this fires.
var ErrBlockedReasonRequired = errors.New("blocked: --reason is required")

type BlockedArgs struct {
	DB       *sql.DB `json:"-"`
	RepoID   string  `json:"repo_id"`
	AgentID  string  `json:"agent_id"`
	ItemID   string  `json:"item_id"`
	Reason   string  `json:"reason"`
	ItemsDir string  `json:"items_dir,omitempty"`
}

type BlockedResult struct {
	ItemID string `json:"item_id"`
	Reason string `json:"reason"`
	AtUnix int64  `json:"at_unix"`
}

func Blocked(ctx context.Context, args BlockedArgs) (*BlockedResult, error) {
	if args.Reason == "" {
		return nil, ErrBlockedReasonRequired
	}
	store := claims.New(args.DB, args.RepoID, nil)
	itemPath := ""
	if args.ItemsDir != "" {
		itemPath = findItemPath(args.ItemsDir, args.ItemID)
	}
	if err := store.Blocked(ctx, args.ItemID, args.AgentID, claims.BlockedOpts{
		Reason:   args.Reason,
		ItemPath: itemPath,
	}); err != nil {
		return nil, err
	}
	return &BlockedResult{
		ItemID: args.ItemID,
		Reason: args.Reason,
		AtUnix: time.Now().Unix(),
	}, nil
}

func newBlockedCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "blocked <ITEM-ID>",
		Short: "Mark item blocked: release claim + status: blocked + ensure ## Blocker section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			res, err := Blocked(ctx, BlockedArgs{
				DB:       bc.db,
				RepoID:   bc.repoID,
				AgentID:  bc.agentID,
				ItemID:   itemID,
				Reason:   reason,
				ItemsDir: bc.itemsDir,
			})
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "blocked %s\n", res.ItemID)
				return nil
			}
			if errors.Is(err, ErrBlockedReasonRequired) {
				return fmt.Errorf("--reason is required for blocked")
			}
			if errors.Is(err, claims.ErrNotYours) {
				fmt.Fprintln(cmd.ErrOrStderr(), "not your claim")
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrNotClaimed) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no active claim on %s\n", itemID)
				os.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "blocker description (required)")
	return cmd
}
