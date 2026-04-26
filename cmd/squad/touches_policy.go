package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/touch"
)

// TouchesPolicyArgs is the input for TouchesPolicy. The hook calls the verb
// once per Edit/Write tool invocation; Path is the file Claude is about to
// modify.
type TouchesPolicyArgs struct {
	DB      *sql.DB
	RepoID  string
	AgentID string
	Path    string
	Cfg     config.TouchConfig
}

// hookOutput mirrors the JSON shape Claude Code's PreToolUse hook protocol
// reads from stdout. permissionDecision is one of "allow" | "deny" | "ask";
// additionalContext is injected verbatim into Claude's next turn.
type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName      string `json:"hookEventName"`
	PermissionDecision string `json:"permissionDecision"`
	AdditionalContext  string `json:"additionalContext"`
}

// TouchesPolicy returns the JSON blob the hook should print to stdout. It
// always returns a stringified JSON object; the hook never has to decide
// what to emit. When there is no conflict the blob is `{"conflict":false}`
// (Claude Code ignores unknown top-level keys, so this is a safe no-op).
func TouchesPolicy(ctx context.Context, args TouchesPolicyArgs) (string, error) {
	tr := touch.New(args.DB, args.RepoID)
	conflicts, err := tr.Conflicts(ctx, args.AgentID, args.Path)
	if err != nil {
		return "", err
	}
	if len(conflicts) == 0 {
		return `{"conflict":false}`, nil
	}
	owner := conflicts[0]
	decision := "ask"
	msg := fmt.Sprintf(
		"squad: %s is currently editing %s. Coordinate via squad knock @%s before editing.",
		owner, args.Path, owner,
	)
	if args.Cfg.Enforcement == config.TouchEnforcementDeny &&
		config.AnyGlobMatches(args.Cfg.EnforcementPaths, args.Path) {
		decision = "deny"
		msg = fmt.Sprintf(
			"squad: blocked - %s is editing %s and the touch policy denies concurrent edits on this path. Run squad knock @%s first.",
			owner, args.Path, owner,
		)
	}
	out := hookOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:      "PreToolUse",
			PermissionDecision: decision,
			AdditionalContext:  msg,
		},
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func newTouchesPolicyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "policy <path>",
		Short: "Emit JSON hook decision for an Edit on <path>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			path := strings.TrimSpace(args[0])
			if path == "" {
				return fmt.Errorf("touches policy: path required")
			}
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			out, err := TouchesPolicy(ctx, TouchesPolicyArgs{
				DB:      bc.db,
				RepoID:  bc.repoID,
				AgentID: bc.agentID,
				Path:    path,
				Cfg:     cfg.Touch,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}
