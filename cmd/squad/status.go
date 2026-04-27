package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newStatusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show claimed / ready / blocked / done counts for this repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runStatusWithJSON(asJSON, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit StatusResult as JSON")
	return cmd
}

// StatusArgs is the input for Status. RepoRoot defaults to the cwd when
// empty (CLI use); MCP callers pass it explicitly.
type StatusArgs struct {
	RepoRoot string
}

// StatusResult reports per-repo item counts. ClaimedBy maps agent_id to
// the sorted list of item_ids that agent currently holds; populated only
// when at least one claim row exists, with `omitempty` so the no-claim
// JSON shape stays as-was.
type StatusResult struct {
	Claimed   int                 `json:"claimed"`
	Ready     int                 `json:"ready"`
	Blocked   int                 `json:"blocked"`
	Done      int                 `json:"done"`
	ClaimedBy map[string][]string `json:"claimed_by,omitempty"`
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
	claimedBy := make(map[string][]string)
	if db, derr := store.OpenDefault(); derr == nil {
		defer db.Close()
		if repoID, rerr := repo.IDFor(root); rerr == nil {
			_ = items.Mirror(ctx, db, repoID, w)
			rows, qerr := db.QueryContext(ctx,
				`SELECT item_id, agent_id FROM claims WHERE repo_id = ?`, repoID)
			if qerr == nil {
				defer rows.Close()
				for rows.Next() {
					var id, agent string
					if err := rows.Scan(&id, &agent); err == nil {
						claimed[id] = struct{}{}
						claimedBy[agent] = append(claimedBy[agent], id)
					}
				}
			}
		}
	}
	for agent := range claimedBy {
		sort.Strings(claimedBy[agent])
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

	out := &StatusResult{
		Claimed: len(claimed),
		Ready:   ready,
		Blocked: c.Blocked,
		Done:    c.Done,
	}
	if len(claimedBy) > 0 {
		out.ClaimedBy = claimedBy
	}
	return out, nil
}

func runStatus(_ []string, stdout io.Writer) int {
	return runStatusWithJSON(false, stdout)
}

func runStatusWithJSON(asJSON bool, stdout io.Writer) int {
	res, err := Status(context.Background(), StatusArgs{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 4
		}
		return 0
	}
	fmt.Fprintf(stdout, "claimed: %d\n", res.Claimed)
	// Contention is a multi-agent concern, not a multi-item one — one agent
	// with two claims is noise, two agents with one claim each is the case
	// the operator wants to see.
	if len(res.ClaimedBy) > 1 {
		agents := make([]string, 0, len(res.ClaimedBy))
		for a := range res.ClaimedBy {
			agents = append(agents, a)
		}
		sort.Strings(agents)
		for _, a := range agents {
			fmt.Fprintf(stdout, "  %s: %s\n", a, strings.Join(res.ClaimedBy[a], ", "))
		}
	}
	fmt.Fprintf(stdout, "ready: %d\nblocked: %d\ndone: %d\n",
		res.Ready, res.Blocked, res.Done)
	return 0
}

func contains(m map[string]struct{}, k string) bool {
	_, ok := m[k]
	return ok
}
