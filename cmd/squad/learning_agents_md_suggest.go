package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
)

var (
	ErrDiffPathRequired  = errors.New("--diff is required")
	ErrRationaleRequired = errors.New("--rationale is required")
	ErrDiffFileMissing   = errors.New("diff file missing")
)

type LearningAgentsMdSuggestArgs struct {
	RepoRoot  string `json:"repo_root"`
	DiffPath  string `json:"diff_path"`
	Rationale string `json:"rationale"`
	Slug      string `json:"slug,omitempty"`
	CreatedBy string `json:"created_by"`

	Now func() time.Time `json:"-"`
}

type LearningAgentsMdSuggestResult struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func LearningAgentsMdSuggest(_ context.Context, args LearningAgentsMdSuggestArgs) (*LearningAgentsMdSuggestResult, error) {
	if args.DiffPath == "" {
		return nil, ErrDiffPathRequired
	}
	if strings.TrimSpace(args.Rationale) == "" {
		return nil, ErrRationaleRequired
	}
	clock := args.Now
	if clock == nil {
		clock = time.Now
	}
	diff, err := os.ReadFile(args.DiffPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDiffFileMissing, args.DiffPath)
		}
		return nil, fmt.Errorf("read diff: %w", err)
	}
	slug := args.Slug
	if slug == "" {
		slug = "agents-md-edit"
	}
	now := clock().UTC()
	id := now.Format("20060102T150405Z") + "-" + slug
	dir := filepath.Join(args.RepoRoot, ".squad", "learnings", "agents-md", "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, id+".md")
	if err := os.WriteFile(out, []byte(buildAgentsMdProposal(id, args.CreatedBy, args.Rationale, string(diff), now)), 0o644); err != nil {
		return nil, err
	}
	return &LearningAgentsMdSuggestResult{ID: id, Path: out}, nil
}

func newLearningAgentsMdSuggestCmd() *cobra.Command {
	var diffPath, rationale, slug string
	cmd := &cobra.Command{
		Use:   "agents-md-suggest",
		Short: "Propose a unified-diff change to AGENTS.md (human applies via agents-md-approve)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			agentID, _ := identity.AgentID()
			res, err := LearningAgentsMdSuggest(cmd.Context(), LearningAgentsMdSuggestArgs{
				RepoRoot:  root,
				DiffPath:  diffPath,
				Rationale: rationale,
				Slug:      slug,
				CreatedBy: agentID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&diffPath, "diff", "", "path to a unified-diff file (use `git diff -- AGENTS.md` to produce)")
	cmd.Flags().StringVar(&rationale, "rationale", "", "why this change (≤1 paragraph)")
	cmd.Flags().StringVar(&slug, "slug", "", "short slug for the proposal id")
	return cmd
}

func buildAgentsMdProposal(id, agent, rationale, diff string, now time.Time) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "id: %s\nkind: agents-md-suggestion\ncreated: %s\ncreated_by: %s\nstate: proposed\n---\n\n",
		id, now.UTC().Format("2006-01-02"), agent)
	sb.WriteString("## Rationale\n\n")
	sb.WriteString(strings.TrimSpace(rationale))
	sb.WriteString("\n\n## Diff\n\n```diff\n")
	sb.WriteString(diff)
	if !strings.HasSuffix(diff, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n")
	return sb.String()
}
