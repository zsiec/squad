package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/config"
)

var ErrInvalidEventKind = errors.New("invalid event kind")

var validEventKinds = map[string]struct{}{
	"pre_tool":       {},
	"post_tool":      {},
	"subagent_start": {},
	"subagent_stop":  {},
}

type RecordEventArgs struct {
	DB         *sql.DB
	RepoID     string
	AgentID    string
	SessionID  string
	Kind       string
	Tool       string
	Target     string
	ExitCode   int
	DurationMs int

	Now func() time.Time
}

type RecordEventResult struct {
	ID int64
}

func RecordEvent(ctx context.Context, args RecordEventArgs) (*RecordEventResult, error) {
	if _, ok := validEventKinds[args.Kind]; !ok {
		return nil, fmt.Errorf("%w %q (want pre_tool|post_tool|subagent_start|subagent_stop)", ErrInvalidEventKind, args.Kind)
	}
	now := time.Now
	if args.Now != nil {
		now = args.Now
	}
	res, err := args.DB.ExecContext(ctx,
		`INSERT INTO agent_events (repo_id, agent_id, session_id, ts, event_kind, tool, target, exit_code, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		args.RepoID, args.AgentID, args.SessionID, now().Unix(),
		args.Kind, args.Tool, args.Target, args.ExitCode, args.DurationMs,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &RecordEventResult{ID: id}, nil
}

func newEventCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "event",
		Short:  "Record agent activity events (used by hook scripts)",
		Hidden: true,
	}
	cmd.AddCommand(newEventRecordCmd())
	return cmd
}

func newEventRecordCmd() *cobra.Command {
	var kind, tool, target, session, agent string
	var exitCode, durationMs int
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record a single agent_events row",
		// Hooks fire in the parent Claude Code main loop. A non-zero exit
		// here would block the agent, so all failures are logged and
		// swallowed — fail open at this boundary.
		RunE: func(cmd *cobra.Command, args []string) error {
			sess := session
			if sess == "" {
				sess = os.Getenv("SQUAD_SESSION_ID")
			}
			if err := runEventRecord(cmd.Context(), eventRecordArgs{
				Kind:       kind,
				Tool:       tool,
				Target:     target,
				ExitCode:   exitCode,
				DurationMs: durationMs,
				SessionID:  sess,
				AgentID:    agent,
			}); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "squad event record: %v\n", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "pre_tool|post_tool|subagent_start|subagent_stop (required)")
	cmd.Flags().StringVar(&tool, "tool", "", "tool name (e.g. Bash, Edit, Read)")
	cmd.Flags().StringVar(&target, "target", "", "target path or argument")
	cmd.Flags().IntVar(&exitCode, "exit", 0, "exit code")
	cmd.Flags().IntVar(&durationMs, "duration-ms", 0, "duration in milliseconds")
	cmd.Flags().StringVar(&session, "session", "", "session id (defaults to $SQUAD_SESSION_ID)")
	cmd.Flags().StringVar(&agent, "agent", "", "agent id (defaults to derived identity)")
	return cmd
}

type eventRecordArgs struct {
	Kind       string
	Tool       string
	Target     string
	ExitCode   int
	DurationMs int
	SessionID  string
	AgentID    string
}

func runEventRecord(ctx context.Context, a eventRecordArgs) error {
	bc, err := bootClaimContext(ctx)
	if err != nil {
		return err
	}
	defer bc.Close()
	agentID := a.AgentID
	if agentID == "" {
		agentID = bc.agentID
	}

	target := a.Target
	if root, derr := discoverRepoRoot(); derr == nil {
		cfg, _ := config.Load(root)
		target, _ = redact(target, resolveRedactConfig(cfg.Events))
	}

	_, err = RecordEvent(ctx, RecordEventArgs{
		DB:         bc.db,
		RepoID:     bc.repoID,
		AgentID:    agentID,
		SessionID:  a.SessionID,
		Kind:       a.Kind,
		Tool:       a.Tool,
		Target:     target,
		ExitCode:   a.ExitCode,
		DurationMs: a.DurationMs,
	})
	return err
}
