package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newNextCmd() *cobra.Command {
	var (
		asJSON          bool
		limit           int
		includeClaimed  bool
	)
	cmd := &cobra.Command{
		Use:   "next",
		Short: "List ready items in priority order (excludes items already claimed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runNext(args, cmd.OutOrStdout(), asJSON, limit, includeClaimed); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit ready items as a JSON array")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap rows printed (0 = print all)")
	cmd.Flags().BoolVar(&includeClaimed, "include-claimed", false, "include items currently held by some agent")
	return cmd
}

func runNext(_ []string, stdout io.Writer, asJSON bool, limit int, includeClaimed bool) int {
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

	ready := items.Ready(w, time.Now().UTC())
	if !includeClaimed {
		filtered := ready[:0]
		for _, it := range ready {
			if _, held := claimed[it.ID]; !held {
				filtered = append(filtered, it)
			}
		}
		ready = filtered
	}
	if len(ready) == 0 {
		if asJSON {
			fmt.Fprintln(stdout, "[]")
			return 0
		}
		fmt.Fprintln(os.Stderr, "no ready items")
		return 1
	}

	total := len(ready)
	if limit > 0 && limit < total {
		ready = ready[:limit]
	}

	if asJSON {
		out := make([]map[string]any, 0, len(ready))
		for _, it := range ready {
			out = append(out, map[string]any{
				"id":       it.ID,
				"title":    it.Title,
				"priority": it.Priority,
				"estimate": it.Estimate,
				"area":     it.Area,
			})
		}
		b, err := json.Marshal(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 4
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}

	for _, it := range ready {
		marker := ""
		if strings.HasPrefix(it.ID, "EXAMPLE-") {
			marker = " [tutorial]"
		}
		fmt.Fprintf(stdout, "%-12s %-3s %-8s %s%s\n", it.ID, it.Priority, it.Estimate, it.Title, marker)
	}
	if limit > 0 && total > limit {
		fmt.Fprintf(stdout, "... and %d more (use --limit %d for all)\n", total-limit, total)
	}
	return 0
}
