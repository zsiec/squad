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

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

const unscopedRepoID = "_unscoped"

// MaxAgentIDLen and MaxDisplayNameLen cap the values stored in agents table
// columns. R5/R6 fuzzing landed multi-MB inputs that round-tripped fine but
// would inflate every dashboard render. 256 chars is more than long enough
// for any human display name and any session-derived agent id.
const (
	MaxAgentIDLen     = 256
	MaxDisplayNameLen = 256
)

// ErrAgentIDTaken signals that --as <id> matches an existing agent row owned
// by a different worktree/PID. Re-registering would re-point the row, so the
// caller must pass --force to opt in.
var ErrAgentIDTaken = errors.New("register: agent id already registered")

type RegisterArgs struct {
	As          string `json:"as,omitempty"`
	Name        string `json:"name,omitempty"`
	NoRepoCheck bool   `json:"no_repo_check,omitempty"`
	Force       bool   `json:"force,omitempty"`
}

type RegisterResult struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	RepoID    string `json:"repo_id"`
	CreatedAt int64  `json:"created_at"`
}

func Register(ctx context.Context, args RegisterArgs) (*RegisterResult, error) {
	if len(args.As) > MaxAgentIDLen {
		return nil, fmt.Errorf("--as: id too long (%d bytes, max %d)", len(args.As), MaxAgentIDLen)
	}
	if len(args.Name) > MaxDisplayNameLen {
		return nil, fmt.Errorf("--name: too long (%d bytes, max %d)", len(args.Name), MaxDisplayNameLen)
	}
	if err := store.EnsureHome(); err != nil {
		return nil, err
	}
	dbPath, err := store.DBPath()
	if err != nil {
		return nil, err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	id := args.As
	if id != "" {
		if !args.Force && identity.PersistedAgentID() != id {
			if other, ok, _ := lookupAgent(ctx, db, id); ok {
				return nil, fmt.Errorf(
					"%w: agent id %q is already registered (worktree=%s pid=%d). "+
						"Re-using it from this session would re-point the agent row at you. "+
						"Pass --force if you really mean to replace it, or pick a different id with --as.",
					ErrAgentIDTaken, id, other.Worktree, other.PID)
			}
		}
		if err := identity.WritePersistedAgentID(id); err != nil {
			return nil, err
		}
	} else {
		id, err = identity.AgentID()
		if err != nil {
			return nil, err
		}
		if perr := identity.WritePersistedAgentID(id); perr != nil {
			return nil, perr
		}
	}
	name := args.Name
	if name == "" {
		name = id
	}

	repoID := unscopedRepoID
	if !args.NoRepoCheck {
		root, derr := repo.Discover(identity.DetectWorktree())
		if derr != nil {
			return nil, derr
		}
		remote, _ := repo.ReadRemoteURL(root)
		repoID, err = repo.RegisterRepo(ctx, db, root, remote, filepath.Base(root))
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().Unix()
	if err := upsertAgent(ctx, db, repoID, id, name, identity.DetectWorktree(), os.Getpid()); err != nil {
		return nil, err
	}
	return &RegisterResult{AgentID: id, Name: name, RepoID: repoID, CreatedAt: now}, nil
}

func newRegisterCmd() *cobra.Command {
	var (
		asFlag      string
		nameFlag    string
		noRepoCheck bool
		force       bool
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this agent in the squad global database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterWithOpts(cmd.OutOrStdout(), asFlag, nameFlag, noRepoCheck, force)
		},
	}
	cmd.Flags().StringVar(&asFlag, "as", "", "agent id override (persisted per-session)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "display name (defaults to agent id)")
	cmd.Flags().BoolVar(&noRepoCheck, "no-repo-check", false, "internal — `init` will use this; users normally don't call register directly")
	_ = cmd.Flags().MarkHidden("no-repo-check")
	cmd.Flags().BoolVar(&force, "force", false, "claim --as <id> even if another session currently owns that id")
	return cmd
}

func runRegisterWithOpts(stdout io.Writer, asFlag, nameFlag string, noRepoCheck, force bool) error {
	res, err := Register(context.Background(), RegisterArgs{
		As:          asFlag,
		Name:        nameFlag,
		NoRepoCheck: noRepoCheck,
		Force:       force,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "registered %s\n", res.AgentID)
	return nil
}

type agentRow struct {
	Worktree string
	PID      int
}

// lookupAgent returns the agents row for id, if any. ok is false when the
// id is not registered. Used by `register --as` to detect identity-reuse.
func lookupAgent(ctx context.Context, db *sql.DB, id string) (agentRow, bool, error) {
	var r agentRow
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(worktree,''), COALESCE(pid,0) FROM agents WHERE id = ?`, id,
	).Scan(&r.Worktree, &r.PID)
	if err == sql.ErrNoRows {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	return r, true, nil
}

func upsertAgent(ctx context.Context, db *sql.DB, repoID, id, name, worktree string, pid int) error {
	tx, err := store.BeginImmediate(ctx, db)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active')
		ON CONFLICT(id) DO UPDATE SET
			repo_id      = excluded.repo_id,
			display_name = excluded.display_name,
			worktree     = excluded.worktree,
			pid          = excluded.pid,
			last_tick_at = excluded.last_tick_at,
			status       = 'active'
	`, id, repoID, name, worktree, pid, now, now); err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}
	return tx.Commit()
}
