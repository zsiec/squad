package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
	"github.com/zsiec/squad/internal/workspace"
)

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Cross-repo views: status, next, who, list, forget",
	}
	cmd.AddCommand(newWorkspaceStatusCmd())
	cmd.AddCommand(newWorkspaceNextCmd())
	cmd.AddCommand(newWorkspaceWhoCmd())
	cmd.AddCommand(newWorkspaceListCmd())
	cmd.AddCommand(newWorkspaceForgetCmd())
	return cmd
}

// wsContext bundles the dependencies the workspace subcommands need.
type wsContext struct {
	db        *sql.DB
	currentID string
	agentID   string
}

// openWS opens the global DB and discovers (best-effort) the current repo
// + agent. Failures degrade gracefully: missing repo means no current scope.
func openWS() (*wsContext, error) {
	db, err := store.OpenDefault()
	if err != nil {
		return nil, err
	}
	wd, _ := os.Getwd()
	currentID := ""
	if root, derr := repo.Discover(wd); derr == nil {
		if id, ierr := repo.IDFor(root); ierr == nil {
			currentID = id
		}
	}
	agentID, _ := identity.AgentID(wd)
	return &wsContext{db: db, currentID: currentID, agentID: agentID}, nil
}

func (w *wsContext) Close() { _ = w.db.Close() }

func buildFilter(scope, currentID string) (workspace.Filter, error) {
	switch scope {
	case "", "all":
		return workspace.Filter{Mode: workspace.ScopeAll, CurrentRepoID: currentID}, nil
	case "current":
		if currentID == "" {
			return workspace.Filter{}, fmt.Errorf("--repo current requires being inside a known repo")
		}
		return workspace.Filter{Mode: workspace.ScopeCurrent, CurrentRepoID: currentID}, nil
	case "other":
		if currentID == "" {
			return workspace.Filter{}, fmt.Errorf("--repo other requires being inside a known repo")
		}
		return workspace.Filter{Mode: workspace.ScopeOther, CurrentRepoID: currentID}, nil
	default:
		ids := strings.Split(scope, ",")
		clean := make([]string, 0, len(ids))
		for _, id := range ids {
			if t := strings.TrimSpace(id); t != "" {
				clean = append(clean, t)
			}
		}
		return workspace.Filter{Mode: workspace.ScopeExplicit, ExplicitIDs: clean}, nil
	}
}

func newWorkspaceStatusCmd() *cobra.Command {
	var scope string
	var stale time.Duration
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Per-repo summary table",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := openWS()
			if err != nil {
				return err
			}
			defer ws.Close()
			f, err := buildFilter(scope, ws.currentID)
			if err != nil {
				return err
			}
			f.StaleThreshold = stale
			rows, err := workspace.New(ws.db).Status(context.Background(), f)
			if err != nil {
				return err
			}
			if asJSON {
				return renderStatusJSON(cmd.OutOrStdout(), rows, ws)
			}
			renderStatus(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "repo", "", "scope: all (default), current, other, or comma-list of ids")
	cmd.Flags().DurationVar(&stale, "stale-threshold", 0, "exclude repos older than this")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON with current_repo and per-repo rows")
	return cmd
}

func renderStatusJSON(w io.Writer, rows []workspace.StatusRow, ws *wsContext) error {
	current := ""
	jsonRows := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		jsonRows = append(jsonRows, map[string]any{
			"repo_id":     r.RepoID,
			"remote_url":  r.RemoteURL,
			"root_path":   r.RootPath,
			"in_progress": r.InProgress,
			"ready":       r.Ready,
			"blocked":     r.Blocked,
			"last_active": r.LastActiveAt,
		})
		if r.RepoID == ws.currentID && current == "" {
			current = repoDisplayName(r.RemoteURL, r.RootPath)
		}
	}
	out := map[string]any{
		"current_repo": current,
		"repos":        jsonRows,
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func repoDisplayName(remoteURL, rootPath string) string {
	if remoteURL != "" {
		// Trim e.g. "git@github.com:foo/bar.git" or "https://.../bar.git" → "bar"
		s := remoteURL
		if i := strings.LastIndex(s, "/"); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.LastIndex(s, ":"); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(s, ".git")
		if s != "" {
			return s
		}
	}
	if rootPath != "" {
		return path.Base(rootPath)
	}
	return ""
}

func newWorkspaceNextCmd() *cobra.Command {
	var scope string
	var limit int
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Top P0/P1 items across every repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := openWS()
			if err != nil {
				return err
			}
			defer ws.Close()
			f, err := buildFilter(scope, ws.currentID)
			if err != nil {
				return err
			}
			rows, err := workspace.New(ws.db).Next(context.Background(), f, workspace.NextOptions{Limit: limit})
			if err != nil {
				return err
			}
			renderNext(cmd.OutOrStdout(), rows, ws.agentID)
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "repo", "", "scope: all (default), current, other, or comma-list of ids")
	cmd.Flags().IntVar(&limit, "limit", 10, "max rows to print")
	return cmd
}

func newWorkspaceWhoCmd() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "who",
		Short: "Every agent in every repo with last activity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := openWS()
			if err != nil {
				return err
			}
			defer ws.Close()
			f, err := buildFilter(scope, ws.currentID)
			if err != nil {
				return err
			}
			rows, err := workspace.New(ws.db).Who(context.Background(), f)
			if err != nil {
				return err
			}
			renderWho(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&scope, "repo", "", "scope: all (default), current, other, or comma-list of ids")
	return cmd
}

func newWorkspaceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "All registered repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := openWS()
			if err != nil {
				return err
			}
			defer ws.Close()
			rows, err := workspace.New(ws.db).List(context.Background())
			if err != nil {
				return err
			}
			rewriteCurrentRepoPath(rows, ws.currentID)
			renderList(cmd.OutOrStdout(), rows)
			return nil
		},
	}
	return cmd
}

// rewriteCurrentRepoPath substitutes the row matching the CWD's repo_id
// with the canonical CWD path. Same git remote on two checkouts share a
// repo_id; the DB row records whichever clone last ran init/register, which
// surprises users working from the *other* clone. By preferring the CWD at
// display time, each user sees a path they can recognize.
func rewriteCurrentRepoPath(rows []workspace.ListRow, currentID string) {
	if currentID == "" {
		return
	}
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	root, err := repo.Discover(wd)
	if err != nil {
		return
	}
	for i := range rows {
		if rows[i].RepoID == currentID && rows[i].RootPath != root {
			rows[i].RootPath = root
		}
	}
}

func newWorkspaceForgetCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "forget <repo_id>",
		Short: "Remove a repo from the global DB; refuses on active claims unless --force",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := openWS()
			if err != nil {
				return err
			}
			defer ws.Close()
			return workspace.New(ws.db).Forget(context.Background(), args[0], force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even with active claims")
	return cmd
}

func renderStatus(w io.Writer, rows []workspace.StatusRow) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "REPO\tIN_PROGRESS\tREADY\tBLOCKED\tLAST_TICK")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n",
			shortRepo(r.RemoteURL, r.RepoID), r.InProgress, r.Ready, r.Blocked, ageStr(r.LastTickAge))
	}
}

func renderNext(w io.Writer, rows []workspace.NextRow, currentAgent string) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "no P0/P1 items across known repos.")
		fmt.Fprintln(w, "  (workspace next filters to P0/P1 only;")
		fmt.Fprintln(w, "   for the full ready stack of one repo, run `squad next` from that repo.)")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "REPO\tID\tPRI\tEST\tCLAIM\tTITLE")
	for _, r := range rows {
		var claim string
		switch {
		case r.Claimed == "":
			claim = "open"
		case r.Claimed == currentAgent:
			claim = "yours"
		default:
			claim = "@" + r.Claimed
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			shortRepo(r.RemoteURL, r.RepoID), r.ID, r.Priority, r.Estimate, claim, r.Title)
	}
}

func renderWho(w io.Writer, rows []workspace.WhoRow) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "REPO\tAGENT\tNAME\tCLAIM\tINTENT\tTICK")
	for _, r := range rows {
		intent := r.Intent
		if len(intent) > 40 {
			intent = intent[:40]
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			shortRepo(r.RemoteURL, r.RepoID), r.AgentID, r.DisplayName, r.ClaimItem, intent, ageStr(r.LastTickAge))
	}
}

func renderList(w io.Writer, rows []workspace.ListRow) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "REPO_ID\tREMOTE\tROOT\tACTIVE_CLAIMS\tLAST_ACTIVE")
	for _, r := range rows {
		ts := "—"
		if r.LastActiveAt > 0 {
			ts = time.Unix(r.LastActiveAt, 0).Format("2006-01-02 15:04")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", r.RepoID, r.RemoteURL, r.RootPath, r.ItemCount, ts)
	}
}

func shortRepo(remote, id string) string {
	if remote == "" {
		if len(id) > 8 {
			return id[:8]
		}
		return id
	}
	r := strings.TrimSuffix(remote, ".git")
	if i := strings.LastIndexAny(r, "/:"); i >= 0 {
		return r[i+1:]
	}
	return r
}

func ageStr(d time.Duration) string {
	if d < 0 {
		return "—"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// wsItemSourceForCurrent uses the active items mirror table.
var _ = filepath.Join
