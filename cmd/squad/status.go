package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show claimed / ready / blocked / done counts for this repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runStatus(args, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

// StatusArgs is the input for Status. RepoRoot defaults to the cwd when
// empty (CLI use); MCP callers pass it explicitly.
type StatusArgs struct {
	RepoRoot string
}

// StatusResult reports per-repo item counts.
type StatusResult struct {
	Claimed int `json:"claimed"`
	Ready   int `json:"ready"`
	Blocked int `json:"blocked"`
	Done    int `json:"done"`
}

// Status returns claimed / ready / blocked / done counts for the repo.
// Pure read-side aggregation: walks .squad/items + queries the active
// claims table, with no writes.
func Status(ctx context.Context, args StatusArgs) (*StatusResult, error) {
	root := args.RepoRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd: %w", err)
		}
		discovered, err := repo.Discover(wd)
		if err != nil {
			return nil, fmt.Errorf("find repo: %w", err)
		}
		root = discovered
	}

	squadDir := filepath.Join(root, ".squad")
	w, err := items.Walk(squadDir)
	if err != nil {
		return nil, fmt.Errorf("walk items: %w", err)
	}

	claimed := make(map[string]struct{})
	if db, derr := store.OpenDefault(); derr == nil {
		defer db.Close()
		if repoID, rerr := repo.IDFor(root); rerr == nil {
			_ = items.Mirror(ctx, db, repoID, w)
			rows, qerr := db.QueryContext(ctx,
				`SELECT item_id FROM claims WHERE repo_id = ?`, repoID)
			if qerr == nil {
				defer rows.Close()
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err == nil {
						claimed[id] = struct{}{}
					}
				}
			}
		}
	}

	c := items.Counts(w, time.Now().UTC())

	// Frontmatter status only updates on done/blocked transitions, so an item
	// claimed but not yet closed still has status=open in its file. items.Counts
	// over-counts `ready` by including held items. Subtract DB-active claims so
	// `status` and `next` agree on which items are pickable.
	ready := c.Ready
	if len(claimed) > 0 {
		for _, it := range w.Active {
			if (it.Status == "open" || it.Status == "") && contains(claimed, it.ID) {
				ready--
			}
		}
		if ready < 0 {
			ready = 0
		}
	}

	return &StatusResult{
		Claimed: len(claimed),
		Ready:   ready,
		Blocked: c.Blocked,
		Done:    c.Done,
	}, nil
}

func runStatus(_ []string, stdout io.Writer) int {
	res, err := Status(context.Background(), StatusArgs{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Fprintf(stdout, "claimed: %d\nready: %d\nblocked: %d\ndone: %d\n",
		res.Claimed, res.Ready, res.Blocked, res.Done)
	return 0
}

func contains(m map[string]struct{}, k string) bool {
	_, ok := m[k]
	return ok
}
