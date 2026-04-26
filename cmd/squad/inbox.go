package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newInboxCmd() *cobra.Command {
	var (
		mineFlag      bool
		readyOnlyFlag bool
		rejectedFlag  bool
	)
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "List captured items awaiting triage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rejectedFlag && (mineFlag || readyOnlyFlag) {
				return errors.New("--rejected is mutually exclusive with --mine and --ready-only")
			}
			if code := runInbox(cmd.Context(), inboxOpts{
				Mine: mineFlag, ReadyOnly: readyOnlyFlag, Rejected: rejectedFlag,
			}, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&mineFlag, "mine", false, "filter to items captured by this agent")
	cmd.Flags().BoolVar(&readyOnlyFlag, "ready-only", false, "filter to items that pass Definition of Ready")
	cmd.Flags().BoolVar(&rejectedFlag, "rejected", false, "tail .squad/rejected.log instead of listing inbox")
	return cmd
}

type inboxOpts struct{ Mine, ReadyOnly, Rejected bool }

func runInbox(ctx context.Context, opts inboxOpts, stdout, stderr io.Writer) int {
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
	squadDir := filepath.Join(root, ".squad")
	if opts.Rejected {
		return printRejectedLog(squadDir, stdout, stderr)
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

	rows, err := queryCapturedItems(ctx, db, repoID)
	if err != nil {
		fmt.Fprintf(stderr, "query inbox: %v\n", err)
		return 4
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "inbox: empty. nothing awaiting triage.")
		return 0
	}

	var me string
	if opts.Mine {
		me, _ = identity.AgentID()
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tAGE\tCAPTURED_BY\tDOR\tTITLE")
	n := 0
	for _, r := range rows {
		if opts.Mine && r.CapturedBy != me {
			continue
		}
		it, err := items.Parse(r.Path)
		if err != nil {
			continue
		}
		violations := items.DoRCheck(it)
		dorMark := "PASS"
		if len(violations) > 0 {
			dorMark = "FAIL"
		}
		if opts.ReadyOnly && dorMark == "FAIL" {
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			r.ID, inboxAgeStr(r.CapturedAt), r.CapturedBy, dorMark, it.Title)
		n++
	}
	tw.Flush()
	if n == 0 {
		fmt.Fprintln(stdout, "inbox: no items match the filter.")
	}
	return 0
}

type capturedRow struct {
	ID, Path, CapturedBy string
	CapturedAt           int64
}

func queryCapturedItems(ctx context.Context, db *sql.DB, repoID string) ([]capturedRow, error) {
	q := `SELECT item_id, path, COALESCE(captured_by,''), COALESCE(captured_at,0)
          FROM items WHERE repo_id=? AND status='captured'
          ORDER BY captured_at ASC`
	rows, err := db.QueryContext(ctx, q, repoID)
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

func inboxAgeStr(unix int64) string {
	if unix == 0 {
		return "-"
	}
	d := time.Since(time.Unix(unix, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func printRejectedLog(squadDir string, stdout, stderr io.Writer) int {
	p := filepath.Join(squadDir, "rejected.log")
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stdout, "no rejections logged.")
			return 0
		}
		fmt.Fprintf(stderr, "open log: %v\n", err)
		return 4
	}
	defer f.Close()
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tREJECTED\tBY\tREASON")
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	n := 0
	for sc.Scan() {
		var e struct {
			Ts     int64  `json:"ts"`
			ID     string `json:"id"`
			Reason string `json:"reason"`
			By     string `json:"by"`
		}
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.ID, inboxAgeStr(e.Ts), e.By, e.Reason)
		n++
	}
	tw.Flush()
	if n == 0 {
		fmt.Fprintln(stdout, "no rejections logged.")
	}
	return 0
}
