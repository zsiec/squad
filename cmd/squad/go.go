package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newGoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go",
		Short: "Onboard or resume in one step (register, claim, mailbox flush)",
		Long: `go is the single-command entry point for an agent session.

It does whatever is needed to reach "claim held, AC loaded, mailbox
drained": init the workspace if .squad/ is absent, register the agent
if not already registered, find the top ready item, claim it, print
its acceptance criteria, and flush any pending chat into stdout.

Idempotent — running it twice does not claim two items. If a claim is
already held, go resumes that claim, re-prints its AC, and flushes
the mailbox.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGo(cmd)
		},
	}
	return cmd
}

func runGo(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := ensureSquadInit(wd, out); err != nil {
		return err
	}
	if err := ensureRegistered(out); err != nil {
		return err
	}
	if err := ensureClaim(out); err != nil {
		return err
	}
	return flushMailbox(out)
}

func flushMailbox(out io.Writer) error {
	bc, err := bootClaimContext(context.Background())
	if err != nil {
		return err
	}
	defer bc.Close()
	if code := runTickBody(context.Background(), bc.chat, bc.agentID, false, out); code != 0 {
		return fmt.Errorf("mailbox flush returned exit code %d", code)
	}
	return nil
}

func ensureClaim(out io.Writer) error {
	bc, err := bootClaimContext(context.Background())
	if err != nil {
		return err
	}
	defer bc.Close()

	if held, ok, err := agentHoldsClaim(context.Background(), bc); err != nil {
		return err
	} else if ok {
		fmt.Fprintf(out, "resuming claim on %s\n", held)
		return printItemAC(out, bc.itemsDir, held)
	}

	root, err := repo.Discover(identity.DetectWorktree())
	if err != nil {
		return err
	}
	w, err := items.Walk(filepath.Join(root, ".squad"))
	if err != nil {
		return err
	}
	ready := items.Ready(w, time.Now().UTC())
	ready = reorderByScope(ready, filepath.Join(root, ".squad"), os.Getenv("SQUAD_SPEC"))
	claimedSet, err := loadClaimedSet(context.Background(), bc.db, bc.repoID)
	if err != nil {
		return err
	}
	for _, it := range ready {
		if _, taken := claimedSet[it.ID]; taken {
			continue
		}
		err := bc.store.Claim(context.Background(), it.ID, bc.agentID,
			"squad go auto-claim", nil, false,
			claims.ClaimWithPreflight(bc.itemsDir, bc.doneDir))
		if err == nil {
			fmt.Fprintf(out, "claimed %s: %s\n", it.ID, it.Title)
			return printItemAC(out, bc.itemsDir, it.ID)
		}
		if errors.Is(err, claims.ErrClaimTaken) {
			continue
		}
		return err
	}
	fmt.Fprintln(out, "no ready items — workspace is clear")
	return nil
}

func agentHoldsClaim(ctx context.Context, bc *claimContext) (string, bool, error) {
	var id string
	err := bc.db.QueryRowContext(ctx,
		`SELECT item_id FROM claims WHERE agent_id = ? AND repo_id = ? LIMIT 1`,
		bc.agentID, bc.repoID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func loadClaimedSet(ctx context.Context, db *sql.DB, repoID string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	rows, err := db.QueryContext(ctx,
		`SELECT item_id FROM claims WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func ensureSquadInit(wd string, out io.Writer) error {
	if _, err := os.Stat(filepath.Join(wd, ".squad", "config.yaml")); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintln(out, "no .squad/ found — running squad init --yes")
	return runInit(&cobra.Command{}, initOptions{Yes: true, Dir: wd})
}

func ensureRegistered(out io.Writer) error {
	id, err := identity.AgentID()
	if err != nil {
		return err
	}
	known, err := agentExists(id)
	if err != nil {
		return err
	}
	if known {
		return nil
	}
	return runRegisterWithOpts(out, "", "", false, false)
}

func printItemAC(out io.Writer, itemsDir, itemID string) error {
	path := findItemPath(itemsDir, itemID)
	if path == "" {
		fmt.Fprintf(out, "(item file for %s not found in %s)\n", itemID, itemsDir)
		return nil
	}
	it, err := items.Parse(path)
	if err != nil {
		return err
	}
	if len(it.ACItems) == 0 {
		fmt.Fprintln(out, "(item has no acceptance criteria — sharpen before coding)")
		return nil
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Acceptance criteria:")
	for _, ac := range it.ACItems {
		marker := "[ ]"
		if ac.Checked {
			marker = "[x]"
		}
		fmt.Fprintf(out, "  %s %s\n", marker, ac.Text)
	}
	return nil
}

func pickItemForScope(squadDir, scopeSpec string) (string, error) {
	w, err := items.Walk(squadDir)
	if err != nil {
		return "", err
	}
	ready := items.Ready(w, time.Now().UTC())
	if scopeSpec == "" {
		if len(ready) == 0 {
			return "", nil
		}
		return ready[0].ID, nil
	}
	epicList, _, err := epics.Walk(squadDir)
	if err != nil {
		return "", err
	}
	scoped := map[string]bool{}
	for _, e := range epicList {
		if e.Spec == scopeSpec {
			scoped[e.Name] = true
		}
	}
	for _, it := range ready {
		if scoped[it.Epic] {
			return it.ID, nil
		}
	}
	if len(ready) > 0 {
		return ready[0].ID, nil
	}
	return "", nil
}

func reorderByScope(ready []items.Item, squadDir, scopeSpec string) []items.Item {
	if scopeSpec == "" {
		return ready
	}
	epicList, _, err := epics.Walk(squadDir)
	if err != nil {
		return ready
	}
	scoped := map[string]bool{}
	for _, e := range epicList {
		if e.Spec == scopeSpec {
			scoped[e.Name] = true
		}
	}
	in, out := []items.Item{}, []items.Item{}
	for _, it := range ready {
		if scoped[it.Epic] {
			in = append(in, it)
		} else {
			out = append(out, it)
		}
	}
	return append(in, out...)
}

func agentExists(id string) (bool, error) {
	db, err := store.OpenDefault()
	if err != nil {
		return false, err
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agents WHERE id = ?`, id).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
