package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/repo"
)

// ConcurrencyExceededError signals the agent has already met or exceeded its
// agent.claim_concurrency cap. Carries the values the cobra wrapper renders
// for users.
type ConcurrencyExceededError struct {
	AgentID string
	Held    int
	Cap     int
}

func (e *ConcurrencyExceededError) Error() string {
	return fmt.Sprintf("%s already holds %d claim(s); agent.claim_concurrency=%d", e.AgentID, e.Held, e.Cap)
}

// ClaimHeldError augments claims.ErrClaimTaken with the holding agent (when
// looked up successfully) so callers can render "X is already claimed by Y"
// without re-querying.
type ClaimHeldError struct {
	ItemID string
	Holder string
}

func (e *ClaimHeldError) Error() string {
	if e.Holder != "" {
		return fmt.Sprintf("%s is already claimed by %s", e.ItemID, e.Holder)
	}
	return fmt.Sprintf("%s is already claimed", e.ItemID)
}

func (e *ClaimHeldError) Unwrap() error { return claims.ErrClaimTaken }

type ClaimArgs struct {
	DB             *sql.DB  `json:"-"`
	RepoID         string   `json:"repo_id"`
	AgentID        string   `json:"agent_id"`
	ItemID         string   `json:"item_id"`
	Intent         string   `json:"intent,omitempty"`
	Touches        []string `json:"touches,omitempty"`
	Long           bool     `json:"long,omitempty"`
	ItemsDir       string   `json:"items_dir,omitempty"`
	DoneDir        string   `json:"done_dir,omitempty"`
	ConcurrencyCap int      `json:"concurrency_cap,omitempty"`
}

type ClaimResult struct {
	ItemID    string `json:"item_id"`
	AgentID   string `json:"agent_id"`
	Intent    string `json:"intent,omitempty"`
	ClaimedAt int64  `json:"claimed_at"`
}

func Claim(ctx context.Context, args ClaimArgs) (*ClaimResult, error) {
	if args.ConcurrencyCap > 0 {
		var n int
		if err := args.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM claims WHERE agent_id = ? AND repo_id = ?`,
			args.AgentID, args.RepoID).Scan(&n); err == nil && n >= args.ConcurrencyCap {
			return nil, &ConcurrencyExceededError{
				AgentID: args.AgentID,
				Held:    n,
				Cap:     args.ConcurrencyCap,
			}
		}
	}

	store := claims.New(args.DB, args.RepoID, nil)
	err := store.Claim(ctx, args.ItemID, args.AgentID, args.Intent, args.Touches, args.Long,
		claims.ClaimWithPreflight(args.ItemsDir, args.DoneDir))
	if err == nil {
		return &ClaimResult{
			ItemID:    args.ItemID,
			AgentID:   args.AgentID,
			Intent:    args.Intent,
			ClaimedAt: time.Now().Unix(),
		}, nil
	}
	if errors.Is(err, claims.ErrClaimTaken) {
		holder := lookupClaimHolderDB(ctx, args.DB, args.RepoID, args.ItemID)
		return nil, &ClaimHeldError{ItemID: args.ItemID, Holder: holder}
	}
	return nil, err
}

func lookupClaimHolderDB(ctx context.Context, db *sql.DB, repoID, itemID string) string {
	var agent string
	if err := db.QueryRowContext(ctx,
		`SELECT agent_id FROM claims WHERE item_id = ? AND repo_id = ?`,
		itemID, repoID).Scan(&agent); err != nil {
		return ""
	}
	return agent
}

func newClaimCmd() *cobra.Command {
	var (
		intent  string
		touches string
		long    bool
	)
	cmd := &cobra.Command{
		Use:   "claim <ITEM-ID>",
		Short: "Atomically claim an item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			var touchList []string
			if touches != "" {
				for _, p := range strings.Split(touches, ",") {
					if p = strings.TrimSpace(p); p != "" {
						touchList = append(touchList, p)
					}
				}
			}

			res, err := Claim(ctx, ClaimArgs{
				DB:             bc.db,
				RepoID:         bc.repoID,
				AgentID:        bc.agentID,
				ItemID:         itemID,
				Intent:         intent,
				Touches:        touchList,
				Long:           long,
				ItemsDir:       bc.itemsDir,
				DoneDir:        bc.doneDir,
				ConcurrencyCap: claimConcurrencyCap(),
			})
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "claimed %s\n", res.ItemID)
				return nil
			}
			var capErr *ConcurrencyExceededError
			if errors.As(err, &capErr) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"%s already holds %d claim(s); agent.claim_concurrency=%d in .squad/config.yaml.\n"+
						"  release one first, or raise the cap (set claim_concurrency: 100 to effectively disable).\n",
					capErr.AgentID, capErr.Held, capErr.Cap)
				os.Exit(1)
			}
			var heldErr *ClaimHeldError
			if errors.As(err, &heldErr) {
				if heldErr.Holder != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed by %s\n", heldErr.ItemID, heldErr.Holder)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed\n", heldErr.ItemID)
				}
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrBlockedByOpen) {
				fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrItemNotFound) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no item file found for %q in .squad/items/\n", itemID)
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrItemAlreadyDone) {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s is already done (in .squad/done/) — cannot claim\n", itemID)
				os.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&intent, "intent", "", "short sentence describing intent")
	cmd.Flags().StringVar(&touches, "touches", "", "comma-separated file paths you'll modify")
	cmd.Flags().BoolVar(&long, "long", false, "use the 2h long-running threshold instead of hygiene.stale_claim_minutes")
	return cmd
}

func claimConcurrencyCap() int {
	wd, err := os.Getwd()
	if err != nil {
		return config.DefaultClaimConcurrency
	}
	root, err := repo.Discover(wd)
	if err != nil {
		return config.DefaultClaimConcurrency
	}
	cfg, err := config.Load(root)
	if err != nil {
		return config.DefaultClaimConcurrency
	}
	if cfg.Agent.ClaimConcurrency > 0 {
		return cfg.Agent.ClaimConcurrency
	}
	return config.DefaultClaimConcurrency
}
