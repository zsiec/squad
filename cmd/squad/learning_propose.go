package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/learning"
)

func newLearningProposeCmd() *cobra.Command {
	var title, area, sessionID string
	var paths []string
	cmd := &cobra.Command{
		Use:   "propose <kind> <slug>",
		Short: "Stub a new learning artifact under .squad/learnings/<kind>s/proposed/",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := learning.ParseKind(args[0])
			if err != nil {
				return err
			}
			slug := args[1]
			if !validSlug(slug) {
				return fmt.Errorf("slug %q must be kebab-case", slug)
			}
			if title == "" || area == "" {
				return fmt.Errorf("--title and --area are required")
			}
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			all, werr := learning.Walk(root)
			if werr != nil {
				return werr
			}
			for _, l := range all {
				if l.Slug == slug {
					return fmt.Errorf("learning with slug %q already exists at %s", slug, l.Path)
				}
			}
			path := learning.PathFor(root, kind, learning.StateProposed, slug)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			agentID, _ := identity.AgentID()
			if sessionID == "" {
				sessionID = os.Getenv("SQUAD_SESSION_ID")
			}
			if err := os.WriteFile(path, []byte(stubBody(kind, slug, title, area, agentID, sessionID, paths)), 0o644); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "human-readable title")
	cmd.Flags().StringVar(&area, "area", "", "subsystem area (matches items area)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session id (defaults to $SQUAD_SESSION_ID)")
	cmd.Flags().StringSliceVar(&paths, "paths", nil, "glob(s) for skill auto-load (default: internal/<area>/**)")
	return cmd
}

func validSlug(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

func stubBody(k learning.Kind, slug, title, area, agent, session string, paths []string) string {
	if len(paths) == 0 {
		paths = []string{"internal/" + area + "/**"}
	}
	id := fmt.Sprintf("%s-%s-%s", k, time.Now().UTC().Format("2006-01-02"), slug)
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "id: %s\nkind: %s\nslug: %s\ntitle: %s\narea: %s\n", id, k, slug, title, area)
	sb.WriteString("paths:\n")
	for _, p := range paths {
		fmt.Fprintf(&sb, "  - %s\n", p)
	}
	fmt.Fprintf(&sb, "created: %s\ncreated_by: %s\nsession: %s\nstate: proposed\nevidence: []\nrelated_items: []\n---\n\n",
		time.Now().UTC().Format("2006-01-02"), agent, session)
	switch k {
	case learning.KindGotcha:
		sb.WriteString("## Looks like\n\n_What it appears to be on first read._\n\n")
		sb.WriteString("## Is\n\n_What it actually is, with the evidence that proves it._\n\n")
		sb.WriteString("## So\n\n_The corrective action future agents should take._\n")
	case learning.KindPattern:
		sb.WriteString("## When\n\n_Conditions under which this pattern applies._\n\n")
		sb.WriteString("## Do\n\n_The concrete action / structure._\n\n")
		sb.WriteString("## Why\n\n_The reason this is the right call here, not the abstract reason._\n")
	case learning.KindDeadEnd:
		sb.WriteString("## Tried\n\n_What was attempted._\n\n")
		sb.WriteString("## Doesn't work because\n\n_Concrete failure mode._\n\n")
		sb.WriteString("## Instead\n\n_If known: the working alternative._\n")
	}
	return sb.String()
}
