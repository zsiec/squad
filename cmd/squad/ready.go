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

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

// ReadyArgs is the input for Ready. Empty IDs means "all captured items."
// Promote=true triggers items.Promote on each item that passes DoR; the
// cobra `--check` path leaves it false.
type ReadyArgs struct {
	DB         *sql.DB
	RepoID     string
	AcceptedBy string
	IDs        []string
	Promote    bool
}

// ReadyReport is the per-item DoR finding plus optional promotion outcome.
// SkipReason is set when promotion was requested and DoR passed but the
// item's current status disqualified it (e.g., already open / in_progress);
// the agent caller can distinguish that from a DoR fail without parsing
// Error strings.
type ReadyReport struct {
	ID         string               `json:"id"`
	Status     string               `json:"status"`
	Pass       bool                 `json:"pass"`
	Violations []items.DoRViolation `json:"violations,omitempty"`
	Promoted   bool                 `json:"promoted,omitempty"`
	SkipReason string               `json:"skip_reason,omitempty"`
	Error      string               `json:"error,omitempty"`
}

type ReadyResult struct {
	Reports      []ReadyReport `json:"reports"`
	UnknownIDs   []string      `json:"unknown_ids,omitempty"`
	AnyViolation bool          `json:"any_violation,omitempty"`
}

func Ready(ctx context.Context, args ReadyArgs) (*ReadyResult, error) {
	var (
		rows []capturedRow
		err  error
	)
	if len(args.IDs) == 0 {
		rows, err = queryCapturedItems(ctx, args.DB, args.RepoID)
	} else {
		rows, err = queryItemsByIDs(ctx, args.DB, args.RepoID, args.IDs)
	}
	if err != nil {
		return nil, err
	}

	out := &ReadyResult{}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
		rep := ReadyReport{ID: r.ID}
		it, perr := items.Parse(r.Path)
		if perr != nil {
			rep.Error = perr.Error()
			out.Reports = append(out.Reports, rep)
			out.AnyViolation = true
			continue
		}
		rep.Status = it.Status
		rep.Violations = items.DoRCheck(it)
		rep.Pass = len(rep.Violations) == 0
		if !rep.Pass {
			out.AnyViolation = true
		}
		if args.Promote && rep.Pass {
			switch it.Status {
			case "captured":
				if perr := items.Promote(ctx, args.DB, args.RepoID, r.ID, args.AcceptedBy); perr == nil {
					rep.Promoted = true
					rep.Status = "open"
				} else {
					var dorErr *items.DoRError
					if errors.As(perr, &dorErr) {
						rep.Pass = false
						rep.Violations = dorErr.Violations
						out.AnyViolation = true
					} else {
						rep.Error = perr.Error()
					}
				}
			default:
				rep.SkipReason = "status is " + it.Status + "; only captured items are eligible for promotion"
			}
		}
		out.Reports = append(out.Reports, rep)
	}
	for _, id := range args.IDs {
		if !seen[id] {
			out.UnknownIDs = append(out.UnknownIDs, id)
		}
	}
	return out, nil
}

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
	agentID, _ := identity.AgentID()

	res, err := Ready(ctx, ReadyArgs{
		DB:         db,
		RepoID:     repoID,
		AcceptedBy: agentID,
		IDs:        ids,
	})
	if err != nil {
		fmt.Fprintf(stderr, "query: %v\n", err)
		return 4
	}

	if len(ids) == 0 && len(res.Reports) == 0 {
		fmt.Fprintln(stdout, "no captured items to check.")
		return 0
	}

	for _, id := range res.UnknownIDs {
		fmt.Fprintf(stderr, "%s: not found\n", id)
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tVIOLATIONS")
	for _, r := range res.Reports {
		if r.Error != "" {
			fmt.Fprintf(stderr, "%s: parse: %s\n", r.ID, r.Error)
			continue
		}
		if len(r.Violations) == 0 {
			fmt.Fprintf(tw, "%s\tPASS\t-\n", r.ID)
			continue
		}
		var msgs string
		for i, v := range r.Violations {
			if i > 0 {
				msgs += "; "
			}
			msgs += v.Rule + ":" + v.Message
		}
		fmt.Fprintf(tw, "%s\tFAIL\t%s\n", r.ID, msgs)
	}
	tw.Flush()

	if len(res.UnknownIDs) > 0 {
		return 1
	}
	if strict && res.AnyViolation {
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
