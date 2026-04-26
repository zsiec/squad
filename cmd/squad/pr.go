package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/prmark"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "GitHub pull-request integration",
		Long:  "Maps PRs to squad items via a hidden marker in the PR description.",
	}
	cmd.AddCommand(newPRLinkCmd())
	cmd.AddCommand(newPRCloseCmd())
	return cmd
}

func newPRLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr-link <ITEM-ID>",
		Short: "Record a pending PR <-> item mapping (run before gh pr create)",
		Args:  cobra.ExactArgs(1),
		RunE:  runPRLink,
	}
	cmd.Flags().Bool("write-to-clipboard", false, "copy the marker comment to the system clipboard")
	cmd.Flags().Int("pr", 0, "if set, append the marker to an existing PR via gh pr edit")
	return cmd
}

func newPRCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr-close <PR-NUMBER>",
		Short: "Archive the squad item linked to a merged PR (CI-only)",
		Args:  cobra.ExactArgs(1),
		RunE:  runPRClose,
	}
	cmd.Flags().String("repo-id", "", "explicit repo id (used by CI when run from a fresh checkout)")
	return cmd
}

// PRLinkArgs is the input for PRLink.
type PRLinkArgs struct {
	RepoRoot         string
	ItemID           string
	WriteToClipboard bool
	PR               int
}

// PRLinkResult reports the marker that was generated and what side-effects
// fired (clipboard, gh-pr-edit).
type PRLinkResult struct {
	ItemID           string `json:"item_id"`
	Marker           string `json:"marker"`
	Branch           string `json:"branch"`
	WroteToClipboard bool   `json:"wrote_to_clipboard,omitempty"`
	AppendedToPR     int    `json:"appended_to_pr,omitempty"`
}

// PRLink records a pending PR ↔ item mapping in .squad/pending-prs.json
// and returns the marker to embed in the PR body. Optionally writes the
// marker to the clipboard and/or appends it to an existing PR via gh.
func PRLink(ctx context.Context, args PRLinkArgs) (*PRLinkResult, error) {
	if args.ItemID == "" {
		return nil, fmt.Errorf("pr-link: item id required")
	}
	root := args.RepoRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		discovered, err := repo.Discover(wd)
		if err != nil {
			discovered = wd
		}
		root = discovered
	}
	squadDir := filepath.Join(root, ".squad")
	if _, _, err := items.FindByID(squadDir, args.ItemID); err != nil {
		return nil, fmt.Errorf("item %s not found in %s: %w", args.ItemID, squadDir, err)
	}
	if args.PR > 0 {
		if _, err := exec.LookPath("gh"); err != nil {
			return nil, fmt.Errorf("--pr requires gh CLI on PATH")
		}
	}

	branch := currentBranch(root)
	pendingPath := filepath.Join(squadDir, "pending-prs.json")
	entry := prmark.Entry{
		ItemID:    args.ItemID,
		Branch:    branch,
		CreatedAt: time.Now().UTC(),
	}
	if err := prmark.AppendPending(pendingPath, entry); err != nil {
		return nil, fmt.Errorf("write pending-prs: %w", err)
	}

	res := &PRLinkResult{
		ItemID: args.ItemID,
		Marker: prmark.Format(args.ItemID),
		Branch: branch,
	}
	if args.WriteToClipboard {
		if err := prmark.WriteClipboard(res.Marker); err == nil {
			res.WroteToClipboard = true
		}
	}
	if args.PR > 0 {
		if err := appendMarkerToPR(args.PR, res.Marker); err != nil {
			return res, fmt.Errorf("append to PR #%d: %w", args.PR, err)
		}
		res.AppendedToPR = args.PR
	}
	return res, nil
}

// PRCloseArgs is the input for PRClose.
type PRCloseArgs struct {
	RepoRoot string
	PRNumber string
}

// PRCloseResult reports the outcome of pr-close: which item was archived,
// or a no-op reason.
type PRCloseResult struct {
	ItemID   string `json:"item_id,omitempty"`
	PRNumber string `json:"pr_number"`
	Archived bool   `json:"archived"`
	NoOp     string `json:"no_op,omitempty"`
}

// PRClose finds the squad item linked to a merged PR (via the marker in
// the PR body) and archives it. Used by the auto-archive workflow.
func PRClose(ctx context.Context, args PRCloseArgs) (*PRCloseResult, error) {
	if n, err := strconv.Atoi(args.PRNumber); err != nil || n <= 0 {
		return nil, fmt.Errorf("pr-close: PR number must be a positive integer, got %q", args.PRNumber)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("gh CLI not found in PATH (required for pr-close)")
	}
	tCtx, cancel := context.WithTimeout(ctx, ghTimeout)
	defer cancel()
	out, err := exec.CommandContext(tCtx, "gh", "pr", "view", args.PRNumber, "--json", "body", "-q", ".body").Output()
	if err != nil {
		if tCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("gh pr view %s timed out after %s", args.PRNumber, ghTimeout)
		}
		return nil, fmt.Errorf("gh pr view %s: %w", args.PRNumber, err)
	}

	itemID := prmark.Extract(string(out))
	if itemID == "" {
		return &PRCloseResult{PRNumber: args.PRNumber, NoOp: "no squad-item marker in PR body"}, nil
	}

	root := args.RepoRoot
	if root == "" {
		wd, _ := os.Getwd()
		discovered, derr := repo.Discover(wd)
		if derr != nil {
			discovered = wd
		}
		root = discovered
	}
	squadDir := filepath.Join(root, ".squad")
	itemPath, inDone, err := items.FindByID(squadDir, itemID)
	if err != nil {
		if errors.Is(err, items.ErrItemNotFound) {
			return &PRCloseResult{ItemID: itemID, PRNumber: args.PRNumber, NoOp: "item not found"}, nil
		}
		return nil, fmt.Errorf("find item %s: %w", itemID, err)
	}
	if inDone {
		return &PRCloseResult{ItemID: itemID, PRNumber: args.PRNumber, NoOp: "item already done"}, nil
	}

	if err := items.RewriteStatus(itemPath, "done", time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("rewrite status: %w", err)
	}
	doneDir := filepath.Join(squadDir, "done")
	movedPath, err := items.MoveToDone(itemPath, doneDir)
	if err != nil {
		return nil, fmt.Errorf("move to done/: %w", err)
	}
	db, derr := store.OpenDefault()
	if derr != nil {
		return nil, fmt.Errorf("open default db for items persist: %w", derr)
	}
	defer db.Close()
	repoID, rerr := repo.IDFor(root)
	if rerr != nil {
		return nil, fmt.Errorf("repo id: %w", rerr)
	}
	parsed, perr := items.Parse(movedPath)
	if perr != nil {
		return nil, fmt.Errorf("parse moved item for persist: %w", perr)
	}
	if err := items.Persist(ctx, db, repoID, parsed, true); err != nil {
		return nil, fmt.Errorf("persist items row: %w", err)
	}
	pendingPath := filepath.Join(squadDir, "pending-prs.json")
	_ = prmark.RemovePending(pendingPath, itemID)
	return &PRCloseResult{ItemID: itemID, PRNumber: args.PRNumber, Archived: true}, nil
}

func runPRLink(cmd *cobra.Command, args []string) error {
	clip, _ := cmd.Flags().GetBool("write-to-clipboard")
	pr, _ := cmd.Flags().GetInt("pr")
	res, err := PRLink(cmd.Context(), PRLinkArgs{
		ItemID:           args[0],
		WriteToClipboard: clip,
		PR:               pr,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), res.Marker)
	fmt.Fprintln(cmd.ErrOrStderr(), "Paste the line above into your PR description (anywhere in the body).")
	if clip {
		if res.WroteToClipboard {
			fmt.Fprintln(cmd.ErrOrStderr(), "(marker copied to clipboard)")
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: clipboard write failed")
		}
	}
	if res.AppendedToPR > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "appended marker to PR #%d\n", res.AppendedToPR)
	}
	return nil
}

// ghTimeout caps any `gh` subprocess call. Without it, a hung gh (auth prompt,
// stalled network, server error mid-stream) would block pr-close indefinitely.
const ghTimeout = 30 * time.Second

func runPRClose(cmd *cobra.Command, args []string) error {
	res, err := PRClose(cmd.Context(), PRCloseArgs{PRNumber: args[0]})
	if err != nil {
		return err
	}
	switch {
	case res.Archived:
		fmt.Fprintf(cmd.OutOrStdout(), "archived %s (PR #%s)\n", res.ItemID, res.PRNumber)
	case res.NoOp == "no squad-item marker in PR body":
		fmt.Fprintln(cmd.ErrOrStderr(), "no squad-item marker in PR #"+res.PRNumber+" — skipping.")
	case res.NoOp == "item not found":
		fmt.Fprintf(cmd.OutOrStdout(), "item %s not found — nothing to archive.\n", res.ItemID)
	case res.NoOp == "item already done":
		fmt.Fprintf(cmd.OutOrStdout(), "item %s already done — nothing to do.\n", res.ItemID)
	}
	return nil
}

func currentBranch(repoRoot string) string {
	out, err := exec.Command("git", "-C", repoRoot, "symbolic-ref", "--quiet", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func appendMarkerToPR(prNum int, marker string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found in PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	current, err := exec.CommandContext(ctx, "gh", "pr", "view", fmt.Sprint(prNum), "--json", "body", "-q", ".body").Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("gh pr view %d timed out after %s", prNum, ghTimeout)
		}
		return fmt.Errorf("read PR body: %w", err)
	}
	body := strings.TrimRight(string(current), "\n") + "\n\n" + marker + "\n"
	editCtx, editCancel := context.WithTimeout(context.Background(), ghTimeout)
	defer editCancel()
	c := exec.CommandContext(editCtx, "gh", "pr", "edit", fmt.Sprint(prNum), "--body-file", "-")
	c.Stdin = strings.NewReader(body)
	out, err := c.CombinedOutput()
	if err != nil {
		if editCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("gh pr edit %d timed out after %s", prNum, ghTimeout)
		}
		return fmt.Errorf("gh pr edit: %w (%s)", err, out)
	}
	return nil
}
