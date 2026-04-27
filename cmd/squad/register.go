package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	// Capabilities is the agent's declared capability set. Populated from
	// `squad register --capability` (CLI) or the equivalent MCP arg.
	Capabilities []string `json:"capabilities,omitempty"`
	// SetCapabilities distinguishes "operator did not pass --capability,
	// preserve the prior set" from "operator passed an empty set, clear
	// to []". Without this sentinel, every implicit re-register from
	// `squad go` would silently wipe the capability column.
	SetCapabilities bool `json:"-"`
}

type RegisterResult struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	RepoID    string `json:"repo_id"`
	CreatedAt int64  `json:"created_at"`
}

func Register(ctx context.Context, args RegisterArgs) (*RegisterResult, []string, error) {
	var warnings []string
	if len(args.As) > MaxAgentIDLen {
		return nil, nil, fmt.Errorf("--as: id too long (%d bytes, max %d)", len(args.As), MaxAgentIDLen)
	}
	if len(args.Name) > MaxDisplayNameLen {
		return nil, nil, fmt.Errorf("--name: too long (%d bytes, max %d)", len(args.Name), MaxDisplayNameLen)
	}
	if err := store.EnsureHome(); err != nil {
		return nil, nil, err
	}
	dbPath, err := store.DBPath()
	if err != nil {
		return nil, nil, err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	id := args.As
	if id != "" {
		if !args.Force && identity.PersistedAgentID() != id {
			if other, ok, _ := lookupAgent(ctx, db, id); ok {
				return nil, nil, fmt.Errorf(
					"%w: agent id %q is already registered (worktree=%s pid=%d) — "+
						"re-using it from this session would re-point the agent row at you; "+
						"pass --force if you really mean to replace it, or pick a different id with --as",
					ErrAgentIDTaken, id, other.Worktree, other.PID)
			}
		}
		if err := identity.WritePersistedAgentID(id); err != nil {
			return nil, nil, err
		}
	} else {
		id, err = identity.AgentID()
		if err != nil {
			return nil, nil, err
		}
		if perr := identity.WritePersistedAgentID(id); perr != nil {
			return nil, nil, perr
		}
	}
	name := args.Name
	if name == "" {
		name = id
	}

	ourWorktree := identity.DetectWorktree()
	ourPid := os.Getpid()
	warnings = appendCollisionWarning(ctx, db, id, ourWorktree, ourPid, warnings)

	repoID := unscopedRepoID
	if !args.NoRepoCheck {
		root, derr := repo.Discover(ourWorktree)
		if derr != nil {
			return nil, nil, derr
		}
		remote, _ := repo.ReadRemoteURL(root)
		repoID, err = repo.RegisterRepo(ctx, db, root, remote, filepath.Base(root))
		if err != nil {
			return nil, nil, err
		}
	}

	now := time.Now().Unix()
	caps := normalizeCapabilities(args.Capabilities)
	if err := upsertAgent(ctx, db, repoID, id, name, ourWorktree, ourPid, caps, args.SetCapabilities); err != nil {
		return nil, nil, err
	}
	return &RegisterResult{AgentID: id, Name: name, RepoID: repoID, CreatedAt: now}, warnings, nil
}

// normalizeCapabilities lowercases, trims, dedupes, and sorts the input so
// re-registering with the same set in any order produces a byte-identical
// capabilities column. The replace-on-re-register semantic depends on this:
// the operator passes the full current shape every call.
func normalizeCapabilities(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

const identityCollisionWindow = 60 // seconds

func appendCollisionWarning(ctx context.Context, db *sql.DB, id, ourWorktree string, ourPid int, warnings []string) []string {
	existing, ok, err := lookupAgent(ctx, db, id)
	if err != nil || !ok {
		return warnings
	}
	if existing.PID == ourPid || existing.Worktree == ourWorktree {
		return warnings
	}
	if time.Now().Unix()-existing.LastTickAt > identityCollisionWindow {
		return warnings
	}
	return append(warnings, fmt.Sprintf(
		"squad: identity collision detected — another live session is registered as %q "+
			"(worktree=%s pid=%d). Re-registering will re-point the agent row at this session. "+
			"Set SQUAD_SESSION_ID=<unique> in this shell to give it a distinct id.",
		id, existing.Worktree, existing.PID,
	))
}

func newRegisterCmd() *cobra.Command {
	var (
		asFlag       string
		nameFlag     string
		noRepoCheck  bool
		force        bool
		capabilities []string
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this agent in the squad global database",
		RunE: func(cmd *cobra.Command, args []string) error {
			setCaps := cmd.Flags().Changed("capability")
			return runRegisterWithOpts(cmd.OutOrStdout(), cmd.ErrOrStderr(), asFlag, nameFlag, noRepoCheck, force, capabilities, setCaps)
		},
	}
	cmd.Flags().StringVar(&asFlag, "as", "", "agent id override (persisted per-session)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "display name (defaults to agent id)")
	cmd.Flags().BoolVar(&noRepoCheck, "no-repo-check", false, "internal — `init` will use this; users normally don't call register directly")
	_ = cmd.Flags().MarkHidden("no-repo-check")
	cmd.Flags().BoolVar(&force, "force", false, "claim --as <id> even if another session currently owns that id")
	cmd.Flags().StringSliceVar(&capabilities, "capability", nil, "declare an agent capability tag; repeatable. Replaces the prior set on re-register.")
	return cmd
}

func runRegisterWithOpts(stdout, stderr io.Writer, asFlag, nameFlag string, noRepoCheck, force bool, capabilities []string, setCapabilities bool) error {
	res, warnings, err := Register(context.Background(), RegisterArgs{
		As:              asFlag,
		Name:            nameFlag,
		NoRepoCheck:     noRepoCheck,
		Force:           force,
		Capabilities:    capabilities,
		SetCapabilities: setCapabilities,
	})
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintln(stderr, w)
	}
	fmt.Fprintf(stdout, "registered %s\n", res.AgentID)
	return nil
}

type agentRow struct {
	Worktree   string
	PID        int
	LastTickAt int64
}

// lookupAgent returns the agents row for id, if any. ok is false when the
// id is not registered. Used by `register --as` to detect identity-reuse
// and by the auto-derived collision check.
func lookupAgent(ctx context.Context, db *sql.DB, id string) (agentRow, bool, error) {
	var r agentRow
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(worktree,''), COALESCE(pid,0), COALESCE(last_tick_at,0) FROM agents WHERE id = ?`, id,
	).Scan(&r.Worktree, &r.PID, &r.LastTickAt)
	if err == sql.ErrNoRows {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	return r, true, nil
}

// upsertAgent writes or updates the agents row. setCapabilities = false
// preserves the prior capabilities column on re-register (the common case
// for `squad go`'s implicit registration); true writes the supplied set
// (empty slice → "[]"). The split exists so `squad go` can refresh the
// agent row every session without nuking what the operator declared
// during a real `squad register --capability` call.
func upsertAgent(ctx context.Context, db *sql.DB, repoID, id, name, worktree string, pid int, capabilities []string, setCapabilities bool) error {
	tx, err := store.BeginImmediate(ctx, db)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().Unix()
	capsJSON := []byte("[]")
	if len(capabilities) > 0 {
		b, err := json.Marshal(capabilities)
		if err != nil {
			return fmt.Errorf("marshal capabilities: %w", err)
		}
		capsJSON = b
	}
	updateCapsClause := ""
	if setCapabilities {
		updateCapsClause = ", capabilities = excluded.capabilities"
	}
	stmt := `
		INSERT INTO agents (id, repo_id, display_name, worktree, pid, started_at, last_tick_at, status, capabilities)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active', ?)
		ON CONFLICT(id) DO UPDATE SET
			repo_id      = excluded.repo_id,
			display_name = excluded.display_name,
			worktree     = excluded.worktree,
			pid          = excluded.pid,
			last_tick_at = excluded.last_tick_at,
			status       = 'active'` + updateCapsClause
	if _, err := tx.ExecContext(ctx, stmt,
		id, repoID, name, worktree, pid, now, now, string(capsJSON)); err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}
	return tx.Commit()
}
