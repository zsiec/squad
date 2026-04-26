package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newReadyCmd() *cobra.Command {
	var (
		checkFlag  bool
		strictFlag bool
	)
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "Lint Definition of Ready for captured items",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !checkFlag {
				return errors.New("squad ready: --check is required (this verb only lints today)")
			}
			if code := runReadyCheck(cmd.Context(), args, strictFlag, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&checkFlag, "check", false, "lint Definition of Ready for captured items (required today)")
	cmd.Flags().BoolVar(&strictFlag, "strict", false, "exit non-zero if any item has violations")
	return cmd
}

func runReadyCheck(ctx context.Context, ids []string, strict bool, stdout, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "getwd: %v\n", err)
		return 4
	}
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(stderr, "find repo: %v\n", err)
		return 4
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		fmt.Fprintf(stderr, "repo id: %v\n", err)
		return 4
	}
	db, err := store.OpenDefault()
	if err != nil {
		fmt.Fprintf(stderr, "open store: %v\n", err)
		return 4
	}
	defer db.Close()

	var rows []capturedRow
	if len(ids) == 0 {
		rows, err = queryCapturedItems(ctx, db, repoID)
		if err != nil {
			fmt.Fprintf(stderr, "query: %v\n", err)
			return 4
		}
		if len(rows) == 0 {
			fmt.Fprintln(stdout, "no captured items to check.")
			return 0
		}
	} else {
		rows, err = queryItemsByIDs(ctx, db, repoID, ids)
		if err != nil {
			fmt.Fprintf(stderr, "query: %v\n", err)
			return 4
		}
	}

	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
	}
	unknownMissing := false
	for _, id := range ids {
		if !seen[id] {
			fmt.Fprintf(stderr, "%s: not found\n", id)
			unknownMissing = true
		}
	}

	anyViolation := false
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tVIOLATIONS")
	for _, r := range rows {
		it, err := items.Parse(r.Path)
		if err != nil {
			fmt.Fprintf(stderr, "%s: parse: %v\n", r.ID, err)
			anyViolation = true
			continue
		}
		violations := items.DoRCheck(it)
		if len(violations) == 0 {
			fmt.Fprintf(tw, "%s\tPASS\t-\n", r.ID)
			continue
		}
		anyViolation = true
		var msgs string
		for i, v := range violations {
			if i > 0 {
				msgs += "; "
			}
			msgs += v.Rule + ":" + v.Message
		}
		fmt.Fprintf(tw, "%s\tFAIL\t%s\n", r.ID, msgs)
	}
	tw.Flush()

	if unknownMissing {
		return 1
	}
	if strict && anyViolation {
		return 1
	}
	return 0
}

func queryItemsByIDs(ctx context.Context, db *sql.DB, repoID string, ids []string) ([]capturedRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := ""
	args := []any{repoID}
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	q := `SELECT item_id, path, COALESCE(captured_by,''), COALESCE(captured_at,0)
          FROM items WHERE repo_id=? AND item_id IN (` + placeholders + `)`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []capturedRow
	for rows.Next() {
		var r capturedRow
		if err := rows.Scan(&r.ID, &r.Path, &r.CapturedBy, &r.CapturedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
