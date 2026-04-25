package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/repo"
)

func lookupClaimHolder(ctx context.Context, bc *claimContext, itemID string) string {
	var agent string
	err := bc.db.QueryRowContext(ctx,
		`SELECT agent_id FROM claims WHERE item_id = ? AND repo_id = ?`,
		itemID, bc.repoID).Scan(&agent)
	if err != nil {
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

			// Enforce agent.claim_concurrency from config. Documented as
			// default 1 since Phase 6 but QA round 4 surfaced that nothing
			// actually checked it.
			if cap := claimConcurrencyCap(); cap > 0 {
				var n int
				if err := bc.db.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM claims WHERE agent_id = ? AND repo_id = ?`,
					bc.agentID, bc.repoID).Scan(&n); err == nil && n >= cap {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"%s already holds %d claim(s); agent.claim_concurrency=%d in .squad/config.yaml.\n"+
							"  release one first, or raise the cap (set claim_concurrency: 100 to effectively disable).\n",
						bc.agentID, n, cap)
					os.Exit(1)
				}
			}

			err = bc.store.Claim(ctx, itemID, bc.agentID, intent, touchList, long,
				claims.ClaimWithPreflight(bc.itemsDir, bc.doneDir))
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "claimed %s\n", itemID)
				return nil
			}
			if errors.Is(err, claims.ErrClaimTaken) {
				holder := lookupClaimHolder(ctx, bc, itemID)
				if holder != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed by %s\n", itemID, holder)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s is already claimed\n", itemID)
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
