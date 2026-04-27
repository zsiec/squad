package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/intake"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newIntakeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "intake",
		Short: "Interview-driven intake: capture a rough idea via Q&A and commit a validated bundle",
	}
	cmd.AddCommand(newIntakeNewCmd())
	cmd.AddCommand(newIntakeRefineCmd())
	cmd.AddCommand(newIntakeListCmd())
	cmd.AddCommand(newIntakeStatusCmd())
	cmd.AddCommand(newIntakeCancelCmd())
	cmd.AddCommand(newIntakeCommitCmd())
	return cmd
}

// intakeCtx is the common setup every intake subcommand needs: the
// configured DB, the resolved repo id, the caller's agent id, the squad
// directory under the discovered repo root, plus the cleanup that closes
// the DB. Any failure here returns a positive exit code and a stderr line.
type intakeCtx struct {
	db       *sql.DB
	repoID   string
	agentID  string
	squadDir string
	close    func()
}

func newIntakeContext(stderr io.Writer) (intakeCtx, int) {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "intake: getwd: %v\n", err)
		return intakeCtx{}, 4
	}
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(stderr, "intake: find repo: %v\n", err)
		return intakeCtx{}, 4
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		fmt.Fprintf(stderr, "intake: repo id: %v\n", err)
		return intakeCtx{}, 4
	}
	agentID, _ := identity.AgentID()
	if strings.TrimSpace(agentID) == "" {
		fmt.Fprintln(stderr, "intake: no agent id (run 'squad register' first)")
		return intakeCtx{}, 4
	}
	db, err := store.OpenDefault()
	if err != nil {
		fmt.Fprintf(stderr, "intake: open store: %v\n", err)
		return intakeCtx{}, 4
	}
	return intakeCtx{
		db:       db,
		repoID:   repoID,
		agentID:  agentID,
		squadDir: filepath.Join(root, ".squad"),
		close:    func() { _ = db.Close() },
	}, 0
}

const skillBriefing = `Read .squad/intake-checklist.yaml (or the embedded default) for the
required-fields contract. Drive the interview one question at a time;
call 'squad intake commit <id> --bundle <path-to-json>' when the user
signs off.`

func newIntakeNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <idea...>",
		Short: "Open a green-field intake session for a rough idea",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runIntakeNew(cmd.Context(), args, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runIntakeNew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	ic, code := newIntakeContext(stderr)
	if code != 0 {
		return code
	}
	defer ic.close()

	idea := strings.Join(args, " ")
	s, _, resumed, err := intake.Open(ctx, ic.db, intake.OpenParams{
		RepoID: ic.repoID, AgentID: ic.agentID, Mode: intake.ModeNew,
		IdeaSeed: idea, SquadDir: ic.squadDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "intake new: %v\n", err)
		return 4
	}
	if resumed {
		fmt.Fprintf(stdout, "resumed existing session %s\n", s.ID)
	} else {
		fmt.Fprintf(stdout, "opened session %s\n", s.ID)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, skillBriefing)
	return 0
}

func newIntakeRefineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refine <item-id>",
		Short: "Open a refine-mode intake session backed by an existing captured item",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runIntakeRefine(cmd.Context(), args[0], cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runIntakeRefine(ctx context.Context, itemID string, stdout, stderr io.Writer) int {
	ic, code := newIntakeContext(stderr)
	if code != 0 {
		return code
	}
	defer ic.close()

	s, snap, resumed, err := intake.Open(ctx, ic.db, intake.OpenParams{
		RepoID: ic.repoID, AgentID: ic.agentID, Mode: intake.ModeRefine,
		RefineItemID: itemID, SquadDir: ic.squadDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "intake refine: %v\n", err)
		return 4
	}
	if resumed {
		fmt.Fprintf(stdout, "resumed refine session %s (item %s)\n", s.ID, itemID)
	} else {
		fmt.Fprintf(stdout, "opened refine session %s for %s\n", s.ID, itemID)
	}
	if snap.ID != "" {
		fmt.Fprintf(stdout, "  title:  %s\n", snap.Title)
		fmt.Fprintf(stdout, "  area:   %s\n", snap.Area)
		fmt.Fprintf(stdout, "  status: %s\n", snap.Status)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, skillBriefing)
	return 0
}

func newIntakeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List open intake sessions for this agent in this repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runIntakeList(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runIntakeList(ctx context.Context, stdout, stderr io.Writer) int {
	ic, code := newIntakeContext(stderr)
	if code != 0 {
		return code
	}
	defer ic.close()

	rows, err := ic.db.QueryContext(ctx, `
		SELECT id, mode, COALESCE(refine_item_id,''), idea_seed, created_at
		FROM intake_sessions
		WHERE repo_id=? AND agent_id=? AND status='open'
		ORDER BY created_at DESC
	`, ic.repoID, ic.agentID)
	if err != nil {
		fmt.Fprintf(stderr, "intake list: %v\n", err)
		return 4
	}
	defer rows.Close()

	any := false
	for rows.Next() {
		var id, mode, refineID, ideaSeed string
		var createdAt int64
		if err := rows.Scan(&id, &mode, &refineID, &ideaSeed, &createdAt); err != nil {
			fmt.Fprintf(stderr, "intake list: scan: %v\n", err)
			return 4
		}
		any = true
		switch mode {
		case intake.ModeRefine:
			fmt.Fprintf(stdout, "%s  refine  %s  %s\n", id, refineID, ideaSeed)
		default:
			fmt.Fprintf(stdout, "%s  new            %s\n", id, ideaSeed)
		}
	}
	if !any {
		fmt.Fprintln(stdout, "no open intake sessions")
	}
	return 0
}

func newIntakeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <session-id>",
		Short: "Print the transcript and remaining required fields for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runIntakeStatus(cmd.Context(), args[0], cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runIntakeStatus(ctx context.Context, sessionID string, stdout, stderr io.Writer) int {
	ic, code := newIntakeContext(stderr)
	if code != 0 {
		return code
	}
	defer ic.close()

	checklist, err := intake.LoadChecklist(ic.squadDir)
	if err != nil {
		fmt.Fprintf(stderr, "intake status: %v\n", err)
		return 4
	}
	res, err := intake.Status(ctx, ic.db, checklist, sessionID, ic.agentID)
	if err != nil {
		fmt.Fprintf(stderr, "intake status: %v\n", err)
		return 4
	}

	fmt.Fprintf(stdout, "session %s  mode=%s  status=%s\n",
		res.Session.ID, res.Session.Mode, res.Session.Status)
	if res.Session.IdeaSeed != "" {
		fmt.Fprintf(stdout, "idea: %s\n", res.Session.IdeaSeed)
	}
	if len(res.Transcript) == 0 {
		fmt.Fprintln(stdout, "(no turns yet)")
	} else {
		fmt.Fprintln(stdout, "transcript:")
		for _, t := range res.Transcript {
			fmt.Fprintf(stdout, "  %d  %-6s  %s\n", t.Seq, t.Role, oneLine(t.Content))
		}
	}
	if len(res.StillRequired) > 0 {
		fmt.Fprintf(stdout, "still required: %s\n", strings.Join(res.StillRequired, ", "))
	} else {
		fmt.Fprintln(stdout, "still required: (none — ready to commit)")
	}
	return 0
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

func newIntakeCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <session-id>",
		Short: "Mark an intake session cancelled (irreversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runIntakeCancel(cmd.Context(), args[0], cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runIntakeCancel(ctx context.Context, sessionID string, stdout, stderr io.Writer) int {
	ic, code := newIntakeContext(stderr)
	if code != 0 {
		return code
	}
	defer ic.close()

	if err := intake.Cancel(ctx, ic.db, sessionID, ic.agentID); err != nil {
		fmt.Fprintf(stderr, "intake cancel: %v\n", err)
		return 4
	}
	fmt.Fprintf(stdout, "cancelled %s\n", sessionID)
	return 0
}

func newIntakeCommitCmd() *cobra.Command {
	var bundlePath string
	var ready bool
	cmd := &cobra.Command{
		Use:   "commit <session-id>",
		Short: "Commit an intake session's bundle to disk and the scoreboard (emergency / scriptable form)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runIntakeCommit(cmd.Context(), args[0], bundlePath, ready, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bundlePath, "bundle", "", "path to the JSON bundle file (required)")
	cmd.Flags().BoolVar(&ready, "ready", false, "promote committed items to ready instead of captured")
	_ = cmd.MarkFlagRequired("bundle")
	return cmd
}

func runIntakeCommit(ctx context.Context, sessionID, bundlePath string, ready bool, stdout, stderr io.Writer) int {
	ic, code := newIntakeContext(stderr)
	if code != 0 {
		return code
	}
	defer ic.close()

	body, err := os.ReadFile(bundlePath)
	if err != nil {
		fmt.Fprintf(stderr, "intake commit: read bundle: %v\n", err)
		return 4
	}
	var bundle intake.Bundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		fmt.Fprintf(stderr, "intake commit: parse bundle: %v\n", err)
		return 4
	}
	res, err := intake.Commit(ctx, ic.db, ic.squadDir, sessionID, ic.agentID, bundle, ready)
	if err != nil {
		fmt.Fprintf(stderr, "intake commit: %v\n", err)
		return 4
	}
	fmt.Fprintf(stdout, "committed %s (shape=%s)\n", sessionID, res.Shape)
	for i, id := range res.ItemIDs {
		fmt.Fprintf(stdout, "  %s  %s\n", id, res.Paths[i])
	}
	return 0
}
