package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

// ErrNoReadyItems signals that NextItem found no ready items matching the
// filter. The cobra wrapper exits 1 in this case (back-compat with the old
// runNext contract).
var ErrNoReadyItems = errors.New("next: no ready items")

type NextArgs struct {
	ItemsDir       string  `json:"items_dir"`
	DoneDir        string  `json:"done_dir"`
	DB             *sql.DB `json:"-"`
	RepoID         string  `json:"repo_id"`
	AgentID        string  `json:"agent_id"`
	Limit          int     `json:"limit,omitempty"`
	IncludeClaimed bool    `json:"include_claimed,omitempty"`
}

type NextRow struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Estimate string `json:"estimate"`
	Area     string `json:"area"`
}

type NextResult struct {
	Items []NextRow `json:"items"`
	// Total is the unfiltered count before Limit truncation. The cobra
	// wrapper uses it to print "... and N more".
	Total int `json:"total"`
}

func NextItem(ctx context.Context, args NextArgs) (NextResult, error) {
	squadDir := filepath.Dir(args.ItemsDir)
	w, err := items.Walk(squadDir)
	if err != nil {
		return NextResult{}, fmt.Errorf("walk items: %w", err)
	}

	claimed := map[string]struct{}{}
	if args.DB != nil && args.RepoID != "" {
		_ = items.Mirror(ctx, args.DB, args.RepoID, w)
		rows, qerr := args.DB.QueryContext(ctx,
			`SELECT item_id FROM claims WHERE repo_id = ?`, args.RepoID)
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

	ready := items.Ready(w, time.Now().UTC())
	if !args.IncludeClaimed {
		filtered := ready[:0]
		for _, it := range ready {
			if _, held := claimed[it.ID]; !held {
				filtered = append(filtered, it)
			}
		}
		ready = filtered
	}
	if len(ready) == 0 {
		return NextResult{}, ErrNoReadyItems
	}

	total := len(ready)
	if args.Limit > 0 && args.Limit < total {
		ready = ready[:args.Limit]
	}
	out := make([]NextRow, 0, len(ready))
	for _, it := range ready {
		out = append(out, NextRow{
			ID:       it.ID,
			Title:    it.Title,
			Priority: it.Priority,
			Estimate: it.Estimate,
			Area:     it.Area,
		})
	}
	return NextResult{Items: out, Total: total}, nil
}

func newNextCmd() *cobra.Command {
	var (
		asJSON         bool
		limit          int
		includeClaimed bool
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

	args := NextArgs{
		ItemsDir:       filepath.Join(root, ".squad", "items"),
		DoneDir:        filepath.Join(root, ".squad", "done"),
		Limit:          limit,
		IncludeClaimed: includeClaimed,
	}
	if db, derr := store.OpenDefault(); derr == nil {
		defer db.Close()
		if repoID, rerr := repo.IDFor(root); rerr == nil {
			args.DB = db
			args.RepoID = repoID
		}
	}

	res, err := NextItem(context.Background(), args)
	if errors.Is(err, ErrNoReadyItems) {
		if asJSON {
			fmt.Fprintln(stdout, "[]")
			return 0
		}
		fmt.Fprintln(os.Stderr, "no ready items")
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}

	if asJSON {
		out := make([]map[string]any, 0, len(res.Items))
		for _, it := range res.Items {
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

	for _, it := range res.Items {
		marker := ""
		if strings.HasPrefix(it.ID, "EXAMPLE-") {
			marker = " [tutorial]"
		}
		fmt.Fprintf(stdout, "%-12s %-3s %-8s %s%s\n", it.ID, it.Priority, it.Estimate, it.Title, marker)
	}
	if limit > 0 && res.Total > limit {
		fmt.Fprintf(stdout, "... and %d more (use --limit %d for all)\n", res.Total-limit, res.Total)
	}
	return 0
}
