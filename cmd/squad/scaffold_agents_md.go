package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/scaffold"
	"github.com/zsiec/squad/internal/specs"
	"github.com/zsiec/squad/internal/store"
)

const (
	agentsMdReadyCap = 5
	agentsMdDoneCap  = 10
)

func newScaffoldAgentsMdCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents-md",
		Short: "Write AGENTS.md from the ledger; CLAUDE.md remains hand-edited",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			repoRoot, err := repo.Discover(wd)
			if err != nil {
				return err
			}
			repoID, err := repo.IDFor(repoRoot)
			if err != nil {
				return err
			}
			squadDir := filepath.Join(repoRoot, ".squad")
			db, err := store.OpenDefault()
			if err != nil {
				return err
			}
			defer db.Close()

			walk, err := items.Walk(squadDir)
			if err != nil {
				return fmt.Errorf("walk items: %w", err)
			}
			specsList, err := specs.Walk(squadDir)
			if err != nil {
				return fmt.Errorf("walk specs: %w", err)
			}
			epicsList, _, err := epics.Walk(squadDir)
			if err != nil {
				return fmt.Errorf("walk epics: %w", err)
			}

			ready := capItems(items.Ready(walk, time.Now()), agentsMdReadyCap)
			done := pickDone(walk.Done, agentsMdDoneCap)
			inflight, err := loadInFlightRows(cmd.Context(), db, repoID, walk.Active)
			if err != nil {
				return fmt.Errorf("load claims: %w", err)
			}

			body := scaffold.RenderAgentsMd(scaffold.AgentsMdData{
				Ready:    ready,
				InFlight: inflight,
				Done:     done,
				Specs:    specsList,
				Epics:    epicsList,
			})

			path := filepath.Join(repoRoot, "AGENTS.md")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
}

// capItems returns the first n items, or all of them if there are
// fewer. Used to apply the AGENTS.md per-section limits without
// re-allocating when no cap is needed.
func capItems(in []items.Item, n int) []items.Item {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

// pickDone returns the n most-recently-updated done items. items.Walk
// returns done in os.ReadDir (alphabetic) order, NOT recency, so we
// sort by Updated DESC here. Updated is the YAML date string in
// `2006-01-02` form; lexicographic compare on that format is recency-
// equivalent for our needs.
func pickDone(done []items.Item, n int) []items.Item {
	out := make([]items.Item, len(done))
	copy(out, done)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Updated != out[j].Updated {
			return out[i].Updated > out[j].Updated
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// loadInFlightRows joins active claims with item titles. Items not on
// disk (deleted-while-claimed) get a placeholder title rather than
// dropping the row — the operator wants to see the orphan.
func loadInFlightRows(ctx context.Context, db *sql.DB, repoID string, active []items.Item) ([]scaffold.InFlightRow, error) {
	titleByID := map[string]string{}
	for _, it := range active {
		titleByID[it.ID] = it.Title
	}
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, agent_id, COALESCE(intent, '') FROM claims WHERE repo_id = ? ORDER BY claimed_at`,
		repoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scaffold.InFlightRow
	for rows.Next() {
		var r scaffold.InFlightRow
		if err := rows.Scan(&r.ItemID, &r.ClaimantID, &r.Intent); err != nil {
			return nil, err
		}
		if t, ok := titleByID[r.ItemID]; ok {
			r.Title = t
		} else {
			r.Title = "(orphan — item file missing)"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
