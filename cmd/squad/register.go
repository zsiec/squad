package main

import (
	"context"
	"database/sql"
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

func newRegisterCmd() *cobra.Command {
	var (
		asFlag      string
		nameFlag    string
		noRepoCheck bool
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this agent in the squad global database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(cmd.OutOrStdout(), asFlag, nameFlag, noRepoCheck)
		},
	}
	cmd.Flags().StringVar(&asFlag, "as", "", "agent id override (persisted per-session)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "display name (defaults to agent id)")
	cmd.Flags().BoolVar(&noRepoCheck, "no-repo-check", false, "internal — `init` will use this; users normally don't call register directly")
	return cmd
}

func runRegister(stdout io.Writer, asFlag, nameFlag string, noRepoCheck bool) error {
	if err := store.EnsureHome(); err != nil {
		return err
	}
	dbPath, err := store.DBPath()
	if err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	id := asFlag
	if id != "" {
		if err := identity.WritePersistedAgentID(id); err != nil {
			return err
		}
	} else {
		id, err = identity.AgentID(identity.DetectWorktree())
		if err != nil {
			return err
		}
	}
	name := nameFlag
	if name == "" {
		name = id
	}

	repoID := unscopedRepoID
	if !noRepoCheck {
		root, derr := repo.Discover(identity.DetectWorktree())
		if derr != nil {
			return derr
		}
		remote, _ := repo.ReadRemoteURL(root)
		repoID, err = repo.RegisterRepo(context.Background(), db, root, remote, filepath.Base(root))
		if err != nil {
			return err
		}
	}

	if err := upsertAgent(context.Background(), db, repoID, id, name, identity.DetectWorktree(), os.Getpid()); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "registered %s\n", id)
	return nil
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
