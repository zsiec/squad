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

func runStatus(_ []string, stdout io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 4
	}
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo: %v\n", err)
		return 4
	}
	squadDir := filepath.Join(root, ".squad")
	w, err := items.Walk(squadDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk items: %v\n", err)
		return 4
	}

	claimed := make(map[string]struct{})
	if db, derr := store.OpenDefault(); derr == nil {
		defer db.Close()
		if repoID, rerr := repo.IDFor(root); rerr == nil {
			_ = items.Mirror(context.Background(), db, repoID, w)
			rows, qerr := db.QueryContext(context.Background(),
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

	fmt.Fprintf(stdout, "claimed: %d\nready: %d\nblocked: %d\ndone: %d\n",
		len(claimed), ready, c.Blocked, c.Done)
	return 0
}

func contains(m map[string]struct{}, k string) bool {
	_, ok := m[k]
	return ok
}
