package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
)

func newLearningAgentsMdSuggestCmd() *cobra.Command {
	var diffPath, rationale, slug string
	cmd := &cobra.Command{
		Use:   "agents-md-suggest",
		Short: "Propose a unified-diff change to AGENTS.md (human applies via agents-md-approve)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if diffPath == "" {
				return fmt.Errorf("--diff is required")
			}
			if strings.TrimSpace(rationale) == "" {
				return fmt.Errorf("--rationale is required")
			}
			diff, err := os.ReadFile(diffPath)
			if err != nil {
				return fmt.Errorf("read diff: %w", err)
			}
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			agentID, _ := identity.AgentID()
			if slug == "" {
				slug = "agents-md-edit"
			}
			id := time.Now().UTC().Format("20060102T150405Z") + "-" + slug
			dir := filepath.Join(root, ".squad", "learnings", "agents-md", "proposed")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			out := filepath.Join(dir, id+".md")
			if err := os.WriteFile(out, []byte(buildAgentsMdProposal(id, agentID, rationale, string(diff))), 0o644); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&diffPath, "diff", "", "path to a unified-diff file (use `git diff -- AGENTS.md` to produce)")
	cmd.Flags().StringVar(&rationale, "rationale", "", "why this change (≤1 paragraph)")
	cmd.Flags().StringVar(&slug, "slug", "", "short slug for the proposal id")
	return cmd
}

func buildAgentsMdProposal(id, agent, rationale, diff string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "id: %s\nkind: agents-md-suggestion\ncreated: %s\ncreated_by: %s\nstate: proposed\n---\n\n",
		id, time.Now().UTC().Format("2006-01-02"), agent)
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
