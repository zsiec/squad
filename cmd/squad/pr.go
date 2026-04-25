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

func runPRLink(cmd *cobra.Command, args []string) error {
	itemID := args[0]

	wd, _ := os.Getwd()
	repoRoot, err := repo.Discover(wd)
	if err != nil {
		repoRoot = wd
	}
	squadDir := filepath.Join(repoRoot, ".squad")

	if _, _, err := items.FindByID(squadDir, itemID); err != nil {
		return fmt.Errorf("item %s not found in %s: %w", itemID, squadDir, err)
	}

	// If --pr was passed, validate gh is available BEFORE we touch any files.
	// Otherwise pr-link --pr 42 with no gh installed would write the pending
	// entry, then fail — leaving an entry that doesn't correspond to a real PR.
	pr, _ := cmd.Flags().GetInt("pr")
	if pr > 0 {
		if _, err := exec.LookPath("gh"); err != nil {
			return fmt.Errorf("--pr requires gh CLI on PATH")
		}
	}

	branch := currentBranch(repoRoot)

	pendingPath := filepath.Join(squadDir, "pending-prs.json")
	entry := prmark.Entry{
		ItemID:    itemID,
		Branch:    branch,
		CreatedAt: time.Now().UTC(),
	}
	if err := prmark.AppendPending(pendingPath, entry); err != nil {
		return fmt.Errorf("write pending-prs: %w", err)
	}

	marker := prmark.Format(itemID)
	fmt.Fprintln(cmd.OutOrStdout(), marker)
	fmt.Fprintln(cmd.ErrOrStderr(), "Paste the line above into your PR description (anywhere in the body).")

	if c, _ := cmd.Flags().GetBool("write-to-clipboard"); c {
		if err := prmark.WriteClipboard(marker); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: clipboard write failed: %v\n", err)
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), "(marker copied to clipboard)")
		}
	}

	if pr > 0 {
		if err := appendMarkerToPR(pr, marker); err != nil {
			return fmt.Errorf("append to PR #%d: %w", pr, err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "appended marker to PR #%d\n", pr)
	}
	return nil
}

// ghTimeout caps any `gh` subprocess call. Without it, a hung gh (auth prompt,
// stalled network, server error mid-stream) would block pr-close indefinitely.
const ghTimeout = 30 * time.Second

func runPRClose(cmd *cobra.Command, args []string) error {
	prNum := args[0]
	// Reject obviously-invalid PR numbers up front with a friendly error,
	// rather than letting gh fail with an opaque message.
	if n, err := strconv.Atoi(prNum); err != nil || n <= 0 {
		return fmt.Errorf("pr-close: PR number must be a positive integer, got %q", prNum)
	}

	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found in PATH (required for pr-close)")
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), ghTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", prNum, "--json", "body", "-q", ".body").Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("gh pr view %s timed out after %s", prNum, ghTimeout)
		}
		return fmt.Errorf("gh pr view %s: %w", prNum, err)
	}

	itemID := prmark.Extract(string(out))
	if itemID == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "no squad-item marker in PR #"+prNum+" — skipping.")
		return nil
	}

	wd, _ := os.Getwd()
	repoRoot, err := repo.Discover(wd)
	if err != nil {
		repoRoot = wd
	}
	squadDir := filepath.Join(repoRoot, ".squad")

	itemPath, inDone, err := items.FindByID(squadDir, itemID)
	if err != nil {
		if errors.Is(err, items.ErrItemNotFound) {
			fmt.Fprintf(cmd.OutOrStdout(), "item %s not found — nothing to archive.\n", itemID)
			return nil
		}
		return fmt.Errorf("find item %s: %w", itemID, err)
	}
	if inDone {
		fmt.Fprintf(cmd.OutOrStdout(), "item %s already done — nothing to do.\n", itemID)
		return nil
	}

	if err := items.RewriteStatus(itemPath, "done", time.Now().UTC()); err != nil {
		return fmt.Errorf("rewrite status: %w", err)
	}
	doneDir := filepath.Join(squadDir, "done")
	if _, err := items.MoveToDone(itemPath, doneDir); err != nil {
		return fmt.Errorf("move to done/: %w", err)
	}

	pendingPath := filepath.Join(squadDir, "pending-prs.json")
	if err := prmark.RemovePending(pendingPath, itemID); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to clear pending-prs entry: %v\n", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "archived %s (PR #%s)\n", itemID, prNum)
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
