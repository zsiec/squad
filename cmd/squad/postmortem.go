package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/postmortem"
)

const postmortemPromptTmpl = `You are running a postmortem for a squad item that closed without
'done'. The detector decided no durable learning artifacts were left
behind during the claim window — your job is to capture the lesson
before the context evaporates.

Item file: {{.ItemPath}}
Item ID: {{.ItemID}}
Claimant agent: {{.AgentID}}
Claim window: {{.ClaimedAt}} → {{.ReleasedAt}} (UTC)

Read the item file end-to-end. Read every chat message on thread
{{.ItemID}} that was posted during the claim window (use the squad
listener-tail or squad history tools). Read the git diff for the
item file across the claim window if it exists.

Then write a 'dead-end' learning artifact via:

    squad learning propose dead-end {{.Slug}} --title "..." --area {{.Area}}

Fill in the template body with: hypotheses tried, ruled-out causes,
evidence collected, what to do differently next time. Do NOT name
agents in failure context — describe the work, not the worker. The
artifact lands in the propose state and surfaces to the operator
via 'squad learning list'.

If after reading the item and chat you genuinely conclude no lesson
is worth capturing (e.g. the work was a no-op release with no
investigation), do NOT propose an artifact — instead post one
'squad fyi' on thread {{.ItemID}} explaining why no postmortem is
warranted. Suppressing a hollow learning is better than filing one.
`

var postmortemTmpl = template.Must(template.New("postmortem").Parse(postmortemPromptTmpl))

// PostmortemArgs is the input for RunPostmortem. Most fields are
// resolved from the cobra wrapper; tests inject their own.
type PostmortemArgs struct {
	DB         *sql.DB `json:"-"`
	RepoID     string  `json:"repo_id"`
	RepoRoot   string  `json:"repo_root"`
	ItemID     string  `json:"item_id"`
	ItemPath   string  `json:"item_path"`
	AgentID    string  `json:"agent_id"`
	ClaimedAt  int64   `json:"claimed_at"`
	ReleasedAt int64   `json:"released_at"`
	Cfg        config.PostmortemConfig
	// PrintOnly returns the rendered prompt without spawning claude.
	// Tests use it; operators can use it to inspect what would be
	// sent before paying the LLM round-trip.
	PrintOnly bool `json:"print_only,omitempty"`
	// Dispatcher is injected for tests. Nil means "spawn claude via
	// exec.LookPath." Production callers leave this nil.
	Dispatcher func(ctx context.Context, prompt string) error `json:"-"`
}

type PostmortemResult struct {
	Decision postmortem.Decision `json:"decision"`
	Prompt   string              `json:"prompt,omitempty"`
	Invoked  bool                `json:"invoked"`
}

// RunPostmortem decides whether to dispatch and (if so) renders the
// prompt and runs the dispatcher. Tests pass PrintOnly or a custom
// Dispatcher; the cobra wrapper uses the default exec("claude") path.
func RunPostmortem(ctx context.Context, args PostmortemArgs) (*PostmortemResult, error) {
	if args.ItemID == "" || args.ItemPath == "" {
		return nil, errors.New("postmortem: ItemID and ItemPath required")
	}
	decision, err := postmortem.Detect(ctx, postmortem.Opts{
		DB:              args.DB,
		RepoID:          args.RepoID,
		ItemID:          args.ItemID,
		AgentID:         args.AgentID,
		ClaimedAt:       args.ClaimedAt,
		ReleasedAt:      args.ReleasedAt,
		RepoRoot:        args.RepoRoot,
		ItemPath:        args.ItemPath,
		Enabled:         args.Cfg.IsEnabled(),
		MinChatMessages: args.Cfg.MinChatMessages,
		MinChatChars:    args.Cfg.MinChatChars,
	})
	if err != nil {
		return nil, err
	}
	res := &PostmortemResult{Decision: decision}
	if !decision.Dispatch {
		return res, nil
	}
	prompt, err := renderPostmortemPrompt(args)
	if err != nil {
		return nil, err
	}
	res.Prompt = prompt
	if args.PrintOnly {
		return res, nil
	}
	dispatch := args.Dispatcher
	if dispatch == nil {
		dispatch = defaultClaudeDispatcher
	}
	if err := dispatch(ctx, prompt); err != nil {
		return nil, err
	}
	res.Invoked = true
	return res, nil
}

func renderPostmortemPrompt(args PostmortemArgs) (string, error) {
	slug := slugForPostmortem(args.ItemID, args.ReleasedAt)
	area := "learning"
	var buf bytes.Buffer
	if err := postmortemTmpl.Execute(&buf, map[string]string{
		"ItemPath":   args.ItemPath,
		"ItemID":     args.ItemID,
		"AgentID":    args.AgentID,
		"ClaimedAt":  time.Unix(args.ClaimedAt, 0).UTC().Format(time.RFC3339),
		"ReleasedAt": time.Unix(args.ReleasedAt, 0).UTC().Format(time.RFC3339),
		"Slug":       slug,
		"Area":       area,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func slugForPostmortem(itemID string, ts int64) string {
	t := time.Unix(ts, 0).UTC()
	return fmt.Sprintf("%s-postmortem-%s", itemID, t.Format("20060102-150405"))
}

func defaultClaudeDispatcher(ctx context.Context, prompt string) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude binary not found in PATH; install Claude Code or pass --print-prompt: %w", err)
	}
	c := exec.CommandContext(ctx, claudePath, "--allowedTools", "squad_learning_propose,squad_say,squad_fyi")
	c.Stdin = bytes.NewBufferString(prompt)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func newPostmortemCmd() *cobra.Command {
	var (
		printOnly bool
		agentID   string
	)
	cmd := &cobra.Command{
		Use:   "postmortem <ITEM-ID>",
		Short: "Run the artifact-presence detector on a closed-without-done claim and (optionally) dispatch a follow-up subagent.",
		Long: "Reads the most recent claim_history release for ITEM-ID, runs the postmortem detector, and " +
			"either prints why dispatch is suppressed or invokes a follow-up subagent (via `claude`) " +
			"that captures the lesson as a learning artifact in the propose state. Pass --print-prompt " +
			"to inspect the rendered prompt without spawning claude.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			repoRoot, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			cfg, _ := config.Load(repoRoot)
			itemPath := findItemPath(bc.itemsDir, itemID)
			if itemPath == "" {
				itemPath = findItemPath(bc.doneDir, itemID)
			}
			if itemPath == "" {
				return fmt.Errorf("%s: item not found in items/ or done/", itemID)
			}
			claimAgent, claimedAt, releasedAt, err := loadLastReleaseFor(ctx, bc.db, bc.repoID, itemID)
			if err != nil {
				return err
			}
			if agentID != "" {
				claimAgent = agentID
			}
			res, err := RunPostmortem(ctx, PostmortemArgs{
				DB:         bc.db,
				RepoID:     bc.repoID,
				RepoRoot:   repoRoot,
				ItemID:     itemID,
				ItemPath:   itemPath,
				AgentID:    claimAgent,
				ClaimedAt:  claimedAt,
				ReleasedAt: releasedAt,
				Cfg:        cfg.Postmortem,
				PrintOnly:  printOnly,
			})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !res.Decision.Dispatch {
				fmt.Fprintf(out, "skip dispatch: %s\n", res.Decision.Reason)
				for _, s := range res.Decision.Signals {
					fmt.Fprintf(out, "  - %s: %s\n", s.Kind, s.Detail)
				}
				return nil
			}
			if printOnly {
				fmt.Fprintf(out, "would dispatch: %s\n\n", res.Decision.Reason)
				_, _ = io.WriteString(out, res.Prompt)
				return nil
			}
			fmt.Fprintf(out, "dispatched postmortem subagent for %s\n", itemID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print-prompt", false, "render the prompt without spawning claude")
	cmd.Flags().StringVar(&agentID, "agent", "", "override claim agent id (defaults to the most recent release)")
	return cmd
}

// loadLastReleaseFor returns the most recent CLOSED-WITHOUT-DONE
// release for itemID. The postmortem is about claims that ended
// without `outcome=done`; an item that was released and later
// re-claimed-and-done would otherwise pick the `done` row and the
// detector would scan the wrong window. A trailing `done` after a
// premature release means the lesson has been resolved by the
// re-claim — no postmortem warranted.
func loadLastReleaseFor(ctx context.Context, db *sql.DB, repoID, itemID string) (string, int64, int64, error) {
	var agent string
	var claimedAt, releasedAt int64
	err := db.QueryRowContext(ctx, `
		SELECT agent_id, claimed_at, released_at
		FROM claim_history
		WHERE repo_id = ? AND item_id = ?
		  AND (outcome IS NULL OR outcome != 'done')
		ORDER BY released_at DESC, id DESC
		LIMIT 1
	`, repoID, itemID).Scan(&agent, &claimedAt, &releasedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, 0, fmt.Errorf("%s: no closed-without-done claim_history rows — item never released without 'done', or never claimed", itemID)
		}
		return "", 0, 0, err
	}
	return agent, claimedAt, releasedAt, nil
}
