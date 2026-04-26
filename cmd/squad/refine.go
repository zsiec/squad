package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newRefineCmd() *cobra.Command {
	var comments string
	cmd := &cobra.Command{
		Use:   "refine [<id>...]",
		Short: "Send a captured item back for refinement, or list items needing refinement",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if code := runRefineList(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
					os.Exit(code)
				}
				return nil
			}
			if strings.TrimSpace(comments) == "" {
				return errors.New("--comments is required when an item id is given")
			}
			if code := runRefineMark(cmd.Context(), args, comments, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&comments, "comments", "", "reviewer feedback to record on the item (required when an id is given)")
	return cmd
}

func runRefineMark(ctx context.Context, ids []string, comments string, stdout, stderr io.Writer) int {
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

	anyFailed := false
	for _, id := range ids {
		err := items.Refine(ctx, db, repoID, id, comments)
		if err == nil {
			fmt.Fprintf(stdout, "refined %s\n", id)
			continue
		}
		anyFailed = true
		switch {
		case errors.Is(err, items.ErrCommentsRequired):
			fmt.Fprintf(stderr, "%s: comments required\n", id)
		case errors.Is(err, items.ErrWrongStatusForRefine):
			fmt.Fprintf(stderr, "%s: %v\n", id, err)
		case errors.Is(err, items.ErrItemNotFound):
			fmt.Fprintf(stderr, "%s: no such item\n", id)
		default:
			fmt.Fprintf(stderr, "%s: %v\n", id, err)
		}
	}
	if anyFailed {
		return 1
	}
	return 0
}

func runRefineList(ctx context.Context, stdout, stderr io.Writer) int {
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

	rows, err := queryNeedsRefinementItems(ctx, db, repoID)
	if err != nil {
		fmt.Fprintf(stderr, "query refine list: %v\n", err)
		return 4
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "refine: no items awaiting refinement.")
		return 0
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tAGE\tCAPTURED_BY\tTITLE")
	for _, r := range rows {
		path := filepath.Join(root, ".squad", "items", filepath.Base(r.Path))
		it, err := items.Parse(path)
		if err != nil {
			it, err = items.Parse(r.Path)
			if err != nil {
				continue
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			r.ID, inboxAgeStr(r.UpdatedAt), r.CapturedBy, it.Title)
	}
	tw.Flush()
	return 0
}

type needsRefinementRow struct {
	ID, Path, CapturedBy string
	UpdatedAt            int64
}

func queryNeedsRefinementItems(ctx context.Context, db *sql.DB, repoID string) ([]needsRefinementRow, error) {
	q := `SELECT item_id, path, COALESCE(captured_by,''), COALESCE(updated_at,0)
	      FROM items WHERE repo_id=? AND status='needs-refinement'
	      ORDER BY updated_at ASC`
	rows, err := db.QueryContext(ctx, q, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []needsRefinementRow
	for rows.Next() {
		var r needsRefinementRow
		if err := rows.Scan(&r.ID, &r.Path, &r.CapturedBy, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
