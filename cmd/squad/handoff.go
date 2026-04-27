package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/worktree"
)

func newHandoffCmd() *cobra.Command {
	var (
		shipped   []string
		inflight  []string
		surprised []string
		unblocks  []string
		note      string
		stdinIn   bool
	)
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Post a handoff brief and release any held claims",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			h := chat.HandoffBody{
				Shipped:     shipped,
				InFlight:    inflight,
				SurprisedBy: surprised,
				Unblocks:    unblocks,
				Note:        note,
			}
			if stdinIn {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				extra := strings.TrimSpace(string(data))
				switch {
				case h.Note != "" && extra != "":
					h.Note = h.Note + "\n\n" + extra
				case extra != "":
					h.Note = extra
				}
			}
			repoRoot, _ := discoverRepoRoot()
			if code := runHandoffBody(ctx, bc.chat, bc.store, bc.db, bc.repoID, repoRoot, bc.agentID, h); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&shipped, "shipped", nil, "item shipped this session (repeatable)")
	cmd.Flags().StringSliceVar(&inflight, "in-flight", nil, "item still in flight (repeatable)")
	cmd.Flags().StringSliceVar(&surprised, "surprised-by", nil, "surprising finding (repeatable)")
	cmd.Flags().StringSliceVar(&unblocks, "unblocks", nil, "item unblocked by this work (repeatable)")
	cmd.Flags().StringVar(&note, "note", "", "free-form note")
	cmd.Flags().BoolVar(&stdinIn, "stdin", false, "append note body from stdin")
	return cmd
}

// HandoffArgs is the input for Handoff. At least one of Shipped, InFlight,
// SurprisedBy, Unblocks, or Note must be non-empty — an empty handoff has
// no body and is rejected.
type HandoffArgs struct {
	Chat        *chat.Chat
	ClaimStore  *claims.Store
	DB          *sql.DB
	RepoID      string
	RepoRoot    string
	AgentID     string
	Shipped     []string
	InFlight    []string
	SurprisedBy []string
	Unblocks    []string
	Note        string
}

// HandoffResult reports the outcome of a successful handoff: the summary
// chat surfaced, plus the count of claims released.
type HandoffResult struct {
	AgentID         string   `json:"agent_id"`
	Summary         string   `json:"summary"`
	ClaimsReleased  int      `json:"claims_released"`
	CleanupWarnings []string `json:"cleanup_warnings,omitempty"`
}

// ErrHandoffEmpty is returned when no handoff fields are populated.
var ErrHandoffEmpty = errors.New("handoff: at least one of shipped, in-flight, surprised-by, unblocks, note required")

// Handoff posts a handoff brief to chat and releases every claim the agent
// currently holds. Pure of writers — callers print the result themselves.
func Handoff(ctx context.Context, args HandoffArgs) (*HandoffResult, error) {
	h := chat.HandoffBody{
		Shipped:     args.Shipped,
		InFlight:    args.InFlight,
		SurprisedBy: args.SurprisedBy,
		Unblocks:    args.Unblocks,
		Note:        args.Note,
	}
	if h.Empty() {
		return nil, ErrHandoffEmpty
	}
	if err := args.Chat.PostHandoff(ctx, args.AgentID, h); err != nil {
		return nil, err
	}
	worktreePaths := lookupAgentWorktrees(ctx, args.DB, args.RepoID, args.AgentID)
	released, err := args.ClaimStore.ReleaseAllCount(ctx, args.AgentID, "handoff")
	if err != nil {
		return nil, err
	}
	var warnings []string
	if args.RepoRoot != "" {
		for item, path := range worktreePaths {
			if err := worktree.Cleanup(args.RepoRoot, path); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %s", item, err))
			}
		}
	}
	return &HandoffResult{
		AgentID:         args.AgentID,
		Summary:         h.Summary(),
		ClaimsReleased:  released,
		CleanupWarnings: warnings,
	}, nil
}

func lookupAgentWorktrees(ctx context.Context, db *sql.DB, repoID, agentID string) map[string]string {
	out := map[string]string{}
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, COALESCE(worktree,'') FROM claims WHERE repo_id = ? AND agent_id = ?`,
		repoID, agentID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			continue
		}
		if path != "" {
			out[id] = path
		}
	}
	return out
}

func runHandoffBody(ctx context.Context, c *chat.Chat, claimStore *claims.Store, db *sql.DB, repoID, repoRoot, agentID string, h chat.HandoffBody) int {
	res, err := Handoff(ctx, HandoffArgs{
		Chat:        c,
		ClaimStore:  claimStore,
		DB:          db,
		RepoID:      repoID,
		RepoRoot:    repoRoot,
		AgentID:     agentID,
		Shipped:     h.Shipped,
		InFlight:    h.InFlight,
		SurprisedBy: h.SurprisedBy,
		Unblocks:    h.Unblocks,
		Note:        h.Note,
	})
	if errors.Is(err, ErrHandoffEmpty) {
		fmt.Fprintln(os.Stderr, "handoff requires at least one --shipped / --in-flight / --surprised-by / --unblocks / --note (or --stdin)")
		return 4
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	for _, w := range res.CleanupWarnings {
		fmt.Fprintf(os.Stderr, "warning: worktree cleanup failed: %s\n", w)
	}
	fmt.Printf("handoff posted by %s (%s)\n", res.AgentID, res.Summary)
	return 0
}
