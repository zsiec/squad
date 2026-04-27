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
		shipped    []string
		inflight   []string
		surprised  []string
		unblocks   []string
		note       string
		stdinIn    bool
		propose    bool
		dryRun     bool
		maxPropose int
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
			opts := handoffOpts{
				ProposeFromSurprises: propose,
				DryRun:               dryRun,
				MaxProposals:         maxPropose,
				ItemsDir:             bc.itemsDir,
				SessionID:            os.Getenv("SQUAD_SESSION_ID"),
			}
			if code := runHandoffBody(ctx, bc.chat, bc.store, bc.db, bc.repoID, repoRoot, bc.agentID, h, opts); code != 0 {
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
	cmd.Flags().BoolVar(&propose, "propose-from-surprises", false, "auto-draft a learning proposal per surprise (explicit --surprised-by, else mined from chat history)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "with --propose-from-surprises: preview proposals without writing stubs")
	cmd.Flags().IntVar(&maxPropose, "max", defaultMaxProposals, "with --propose-from-surprises: cap auto-proposals (warns when clipped)")
	return cmd
}

type handoffOpts struct {
	ProposeFromSurprises bool
	DryRun               bool
	MaxProposals         int
	ItemsDir             string
	SessionID            string
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
	ItemsDir    string
	AgentID     string
	SessionID   string
	Shipped     []string
	InFlight    []string
	SurprisedBy []string
	Unblocks    []string
	Note        string

	ProposeFromSurprises bool
	DryRun               bool
	MaxProposals         int
}

// HandoffResult reports the outcome of a successful handoff: the summary
// chat surfaced, plus the count of claims released.
type HandoffResult struct {
	AgentID          string          `json:"agent_id"`
	Summary          string          `json:"summary"`
	ClaimsReleased   int             `json:"claims_released"`
	CleanupWarnings  []string        `json:"cleanup_warnings,omitempty"`
	Proposals        []proposalDraft `json:"proposals,omitempty"`
	ProposalsClipped bool            `json:"proposals_clipped,omitempty"`
	Tips             []string        `json:"tips,omitempty"`
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

	var (
		proposals []proposalDraft
		clipped   bool
	)
	if args.ProposeFromSurprises {
		surprises, c, gerr := gatherSurprises(ctx, args.DB, args.Chat, args.RepoID, args.AgentID, args.ItemsDir, args.SurprisedBy, args.MaxProposals)
		if gerr == nil {
			clipped = c
			drafts, perr := proposeFromSurprises(ctx, args.RepoRoot, args.AgentID, args.SessionID, surprises, args.DryRun)
			if perr == nil {
				proposals = drafts
			}
		}
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
		AgentID:          args.AgentID,
		Summary:          h.Summary(),
		ClaimsReleased:   released,
		CleanupWarnings:  warnings,
		Proposals:        proposals,
		ProposalsClipped: clipped,
		Tips:             buildHandoffTips(proposals, clipped),
	}, nil
}

func buildHandoffTips(drafts []proposalDraft, clipped bool) []string {
	if len(drafts) == 0 {
		return nil
	}
	tips := make([]string, 0, len(drafts)+2)
	tips = append(tips, fmt.Sprintf("drafted %d learning proposal%s — review with `squad learning list --proposed`", len(drafts), plural(len(drafts))))
	for i, d := range drafts {
		tips = append(tips, fmt.Sprintf("  %d. %s", i+1, d.Path))
	}
	if clipped {
		tips = append(tips, "more candidates exist than --max; switch to `squad learning quick` per surprise to file the rest")
	}
	return tips
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

func runHandoffBody(ctx context.Context, c *chat.Chat, claimStore *claims.Store, db *sql.DB, repoID, repoRoot, agentID string, h chat.HandoffBody, opts handoffOpts) int {
	res, err := Handoff(ctx, HandoffArgs{
		Chat:                 c,
		ClaimStore:           claimStore,
		DB:                   db,
		RepoID:               repoID,
		RepoRoot:             repoRoot,
		ItemsDir:             opts.ItemsDir,
		AgentID:              agentID,
		SessionID:            opts.SessionID,
		Shipped:              h.Shipped,
		InFlight:             h.InFlight,
		SurprisedBy:          h.SurprisedBy,
		Unblocks:             h.Unblocks,
		Note:                 h.Note,
		ProposeFromSurprises: opts.ProposeFromSurprises,
		DryRun:               opts.DryRun,
		MaxProposals:         opts.MaxProposals,
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
	if opts.ProposeFromSurprises {
		printProposals(res.Proposals, res.ProposalsClipped, opts.DryRun)
	}
	return 0
}

func printProposals(drafts []proposalDraft, clipped, dryRun bool) {
	if len(drafts) == 0 {
		fmt.Fprintln(os.Stderr, "no surprises to propose from")
		return
	}
	verb := "drafted"
	if dryRun {
		verb = "would draft"
	}
	fmt.Printf("%s %d learning proposal%s — review with `squad learning list --proposed`\n", verb, len(drafts), plural(len(drafts)))
	for i, d := range drafts {
		if dryRun {
			fmt.Printf("  %d. [dry-run] %s · area=%s · title=%s\n", i+1, d.Slug, d.Area, d.Title)
			continue
		}
		fmt.Printf("  %d. %s\n", i+1, d.Path)
	}
	if clipped {
		fmt.Fprintln(os.Stderr, "warning: more candidates exist than --max; switch to `squad learning quick` per surprise to file the rest")
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
