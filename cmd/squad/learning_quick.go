package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
)

// deriveQuickSlug normalises a one-liner into a kebab-case slug suitable for a
// learning artifact filename. Returns empty when the result would be shorter
// than three characters — the propose path treats empty as "too short to use."
func deriveQuickSlug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.TrimLeft(out, "-0123456789")
	out = strings.TrimRight(out, "-")
	if len(out) > 60 {
		out = strings.TrimRight(out[:60], "-")
	}
	if len(out) < 3 {
		return ""
	}
	return out
}

// inferQuickArea picks an area for a quick-captured learning when the user
// didn't supply one. It scans .squad/done/ for the most recently modified
// item file and returns its area; falls back to "general" when no item is
// readable.
func inferQuickArea(repoRoot string) string {
	const fallback = "general"
	doneDir := filepath.Join(repoRoot, ".squad", "done")
	entries, err := os.ReadDir(doneDir)
	if err != nil {
		return fallback
	}
	var bestPath string
	var bestMod int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if mod := info.ModTime().UnixNano(); mod > bestMod {
			bestMod = mod
			bestPath = filepath.Join(doneDir, e.Name())
		}
	}
	if bestPath == "" {
		return fallback
	}
	parsed, err := items.Parse(bestPath)
	if err != nil || parsed.Area == "" {
		return fallback
	}
	return parsed.Area
}

// quickCollisionMaxSuffix caps the collision-walk so a malformed repo with a
// runaway slug family doesn't loop forever. Nine attempts is plenty — if the
// derived slug already has eight collisions, the one-liner is too generic
// and the user should re-run with a more specific phrase.
const quickCollisionMaxSuffix = 9

// LearningQuickArgs is the input for LearningQuick. RepoRoot, OneLiner, and
// CreatedBy are required; Kind defaults to "gotcha" when empty; SessionID is
// optional.
type LearningQuickArgs struct {
	RepoRoot  string `json:"repo_root"`
	OneLiner  string `json:"one_liner"`
	Kind      string `json:"kind,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	CreatedBy string `json:"created_by"`
}

// LearningQuickResult mirrors LearningProposeResult plus the Tips slice so
// MCP callers see the same follow-up nudge text the cobra path writes to
// stderr. Empty Tips when SQUAD_NO_CADENCE_NUDGES is set.
type LearningQuickResult struct {
	*LearningProposeResult
	Tips []string `json:"tips,omitempty"`
}

// LearningQuick is the shared backend for the cobra `learning quick` and the
// MCP `squad_learning_quick` tool. Returns the proposed-learning result plus
// any nudge tips for the caller to render or discard.
func LearningQuick(ctx context.Context, args LearningQuickArgs) (*LearningQuickResult, error) {
	oneLiner := strings.TrimSpace(args.OneLiner)
	baseSlug := deriveQuickSlug(oneLiner)
	if baseSlug == "" {
		return nil, fmt.Errorf("one-liner %q derives to a slug shorter than 3 chars; pass something more specific", oneLiner)
	}
	if args.RepoRoot == "" {
		return nil, fmt.Errorf("repo_root required")
	}
	kind := args.Kind
	if kind == "" {
		kind = "gotcha"
	}
	area := inferQuickArea(args.RepoRoot)

	slug := baseSlug
	for suffix := 1; suffix <= quickCollisionMaxSuffix; suffix++ {
		if suffix > 1 {
			slug = fmt.Sprintf("%s-%d", baseSlug, suffix)
		}
		res, perr := LearningPropose(ctx, LearningProposeArgs{
			RepoRoot:  args.RepoRoot,
			Kind:      kind,
			Slug:      slug,
			Title:     oneLiner,
			Area:      area,
			SessionID: args.SessionID,
			CreatedBy: args.CreatedBy,
			Via:       "quick",
		})
		if perr == nil {
			out := &LearningQuickResult{LearningProposeResult: res}
			if t := quickFollowupNudgeText(); t != "" {
				out.Tips = []string{t}
			}
			return out, nil
		}
		var coll *SlugCollisionError
		if errors.As(perr, &coll) {
			continue
		}
		return nil, perr
	}
	return nil, fmt.Errorf("slug %q already in use through %s-%d; use `squad learning propose` with an explicit slug", baseSlug, baseSlug, quickCollisionMaxSuffix)
}

func newLearningQuickCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "quick <one-liner>",
		Short: "Stub a learning artifact with one-line capture (defaults kind=gotcha, area inferred)",
		Long: `Frictionless one-line capture. Auto-derives the slug from the one-liner, defaults kind to gotcha, and infers area from the most recently closed item in this repo. Skip the ceremony, edit the stub later.

Use squad learning propose for full control over slug, title, area, and paths.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			agentID, _ := identity.AgentID()
			res, err := LearningQuick(cmd.Context(), LearningQuickArgs{
				RepoRoot:  root,
				OneLiner:  args[0],
				Kind:      kind,
				SessionID: os.Getenv("SQUAD_SESSION_ID"),
				CreatedBy: agentID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Path)
			printQuickFollowupNudge(cmd.ErrOrStderr())
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "gotcha", "learning kind (gotcha, pattern, dead-end)")
	return cmd
}
