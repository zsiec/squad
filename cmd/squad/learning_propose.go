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
	"github.com/zsiec/squad/internal/learning"
)

// ErrSlugCollision signals that a learning with the requested slug already
// exists in any state (proposed/approved/rejected) under any kind. The
// SlugCollisionError carries the existing path so callers can render the
// legacy "already exists at <path>" message.
var ErrSlugCollision = errors.New("learning slug already in use")

type SlugCollisionError struct {
	Slug         string
	ExistingPath string
}

func (e *SlugCollisionError) Error() string {
	return fmt.Sprintf("learning with slug %q already exists at %s", e.Slug, e.ExistingPath)
}

func (e *SlugCollisionError) Is(target error) bool { return target == ErrSlugCollision }

type LearningProposeArgs struct {
	RepoRoot  string   `json:"repo_root"`
	Kind      string   `json:"kind"`
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Area      string   `json:"area"`
	SessionID string   `json:"session_id,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	CreatedBy string   `json:"created_by"`
	// Via, when non-empty, injects a `> captured via squad learning <via>`
	// marker line above the kind-specific section template. Used by the
	// `quick` shorthand so reviewers can see at a glance which proposals
	// still need their template sections filled in.
	Via string `json:"via,omitempty"`

	// Looks, when non-empty for KindGotcha, replaces the placeholder
	// "## Looks like" body with the supplied text verbatim. Used by the
	// surprise-mining handoff path so the agent isn't staring at a blank
	// template after auto-drafting.
	Looks string `json:"looks,omitempty"`

	// RelatedItems and Tags emit `related_items:` and `tags:` lists in
	// the frontmatter. Empty slices render as `[]` for related_items and
	// omit tags entirely (omitempty in the schema). Used by the auto-
	// learning pipeline that fires on blocking review attestations.
	RelatedItems []string `json:"related_items,omitempty"`
	Tags         []string `json:"tags,omitempty"`

	Now func() time.Time `json:"-"`
}

type LearningProposeResult struct {
	Path     string             `json:"path"`
	Learning *learning.Learning `json:"learning"`
}

func LearningPropose(_ context.Context, args LearningProposeArgs) (*LearningProposeResult, error) {
	kind, err := learning.ParseKind(args.Kind)
	if err != nil {
		return nil, err
	}
	if !validSlug(args.Slug) {
		return nil, fmt.Errorf("slug %q must be kebab-case", args.Slug)
	}
	if args.Title == "" || args.Area == "" {
		return nil, fmt.Errorf("--title and --area are required")
	}
	clock := args.Now
	if clock == nil {
		clock = time.Now
	}

	all, werr := learning.Walk(args.RepoRoot)
	if werr != nil {
		return nil, werr
	}
	for _, l := range all {
		if l.Slug == args.Slug {
			return nil, &SlugCollisionError{Slug: args.Slug, ExistingPath: l.Path}
		}
	}

	path := learning.PathFor(args.RepoRoot, kind, learning.StateProposed, args.Slug)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	body := stubBody(kind, args.Slug, args.Title, args.Area, args.CreatedBy, args.SessionID, args.Paths, args.Via, args.Looks, args.RelatedItems, args.Tags, clock())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return nil, err
	}
	parsed, perr := learning.Parse(path)
	if perr != nil {
		return nil, perr
	}
	return &LearningProposeResult{Path: path, Learning: &parsed}, nil
}

func newLearningProposeCmd() *cobra.Command {
	var title, area, sessionID string
	var paths []string
	cmd := &cobra.Command{
		Use:   "propose <kind> <slug>",
		Short: "Stub a new learning artifact under .squad/learnings/<kind>s/proposed/",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			agentID, _ := identity.AgentID()
			session := sessionID
			if session == "" {
				session = os.Getenv("SQUAD_SESSION_ID")
			}
			res, err := LearningPropose(cmd.Context(), LearningProposeArgs{
				RepoRoot:  root,
				Kind:      args[0],
				Slug:      args[1],
				Title:     title,
				Area:      area,
				SessionID: session,
				Paths:     paths,
				CreatedBy: agentID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Path)
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
		if r != '-' && (r < '0' || r > '9') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}

func stubBody(k learning.Kind, slug, title, area, agent, session string, paths []string, via, looks string, relatedItems, tags []string, now time.Time) string {
	if len(paths) == 0 {
		paths = []string{"internal/" + area + "/**"}
	}
	id := fmt.Sprintf("%s-%s-%s", k, now.UTC().Format("2006-01-02"), slug)
	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "id: %s\nkind: %s\nslug: %s\ntitle: %s\narea: %s\n", id, k, slug, title, area)
	sb.WriteString("paths:\n")
	for _, p := range paths {
		fmt.Fprintf(&sb, "  - %s\n", p)
	}
	fmt.Fprintf(&sb, "created: %s\ncreated_by: %s\nsession: %s\nstate: proposed\nevidence: []\n",
		now.UTC().Format("2006-01-02"), agent, session)
	if len(relatedItems) == 0 {
		sb.WriteString("related_items: []\n")
	} else {
		sb.WriteString("related_items:\n")
		for _, ri := range relatedItems {
			fmt.Fprintf(&sb, "  - %s\n", ri)
		}
	}
	if len(tags) > 0 {
		sb.WriteString("tags:\n")
		for _, tg := range tags {
			fmt.Fprintf(&sb, "  - %s\n", tg)
		}
	}
	sb.WriteString("---\n\n")
	if via != "" {
		fmt.Fprintf(&sb, "> captured via squad learning %s\n\n", via)
	}
	switch k {
	case learning.KindGotcha:
		looksBody := "_What it appears to be on first read._"
		if looks != "" {
			looksBody = looks
		}
		fmt.Fprintf(&sb, "## Looks like\n\n%s\n\n", looksBody)
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
