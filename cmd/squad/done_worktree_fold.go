package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/worktree"
)

// foldWorktreeMerge merges the per-claim branch `squad/<itemID>-<agentID>`
// into the configured target branch (default "main"). Tries fast-forward
// first; falls back to a merge commit when the target has advanced. On
// a true merge conflict the merge is aborted and the per-claim branch
// is preserved so the user can resolve manually. The branch itself is
// not deleted here — callers must clean up the worktree first (git
// refuses to delete a branch a worktree is checked out on), then call
// foldDeleteBranch.
//
// No-op when the per-claim branch does not exist (non-worktree claims
// or already-folded branches).
func foldWorktreeMerge(repoRoot, itemID, agentID string) error {
	branch := worktree.BranchName(itemID, agentID)

	if !branchExistsInRepo(repoRoot, branch) {
		return nil
	}

	target, err := loadMergeTargetBranch(repoRoot)
	if err != nil {
		return fmt.Errorf("fold: load target branch: %w", err)
	}

	currentBranch := strings.TrimSpace(execGitOutput(repoRoot, "symbolic-ref", "--short", "HEAD"))
	if currentBranch != target {
		return fmt.Errorf("fold: parent repo is on %q, expected %q — switch with `git switch %s` and re-run `squad done`",
			currentBranch, target, target)
	}

	if _, err := execGit(repoRoot, "merge", "--ff-only", branch); err == nil {
		return nil
	}

	out, err := execGit(repoRoot, "merge", "--no-ff", "-m",
		"squad: fold "+branch+" into "+target, branch)
	if err == nil {
		return nil
	}
	combined := string(out)

	if isMergeConflict(combined) {
		_, _ = execGit(repoRoot, "merge", "--abort")
		return fmt.Errorf("fold: merge of %s into %s hit a conflict — branch preserved; resolve with `git switch %s && git merge %s`, then commit; squad done has already archived the item file:\n%s",
			branch, target, target, branch, strings.TrimSpace(combined))
	}
	return fmt.Errorf("fold: merge of %s into %s failed: %s", branch, target, strings.TrimSpace(combined))
}

// foldDeleteBranch removes the per-claim branch after foldWorktreeMerge
// has folded its commits and the caller has torn the worktree down.
// No-op when the branch is already gone.
func foldDeleteBranch(repoRoot, itemID, agentID string) error {
	branch := worktree.BranchName(itemID, agentID)
	if !branchExistsInRepo(repoRoot, branch) {
		return nil
	}
	if out, err := execGit(repoRoot, "branch", "-d", branch); err != nil {
		return fmt.Errorf("fold: branch deletion of %s failed: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}

func branchExistsInRepo(repoRoot, branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

func loadMergeTargetBranch(repoRoot string) (string, error) {
	root, err := repo.Discover(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: merge_target_branch fell back to %q — repo discover failed: %v\n",
			config.DefaultMergeTargetBranch, err)
		return config.DefaultMergeTargetBranch, nil
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: merge_target_branch fell back to %q — config load failed: %v\n",
			config.DefaultMergeTargetBranch, err)
		return config.DefaultMergeTargetBranch, nil
	}
	if t := strings.TrimSpace(cfg.Agent.MergeTargetBranch); t != "" {
		return t, nil
	}
	return config.DefaultMergeTargetBranch, nil
}

func execGit(repoRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

func execGitOutput(repoRoot string, args ...string) string {
	out, err := execGit(repoRoot, args...)
	if err != nil {
		return ""
	}
	return string(out)
}

// isMergeConflict picks merge-conflict signals out of git's combined
// output. A clean failure (exit non-zero, "CONFLICT" in stderr, or
// "Automatic merge failed") all map here. Unrelated failures (target
// missing, dirty working tree) get the generic error path so the
// user sees git's verbatim message.
func isMergeConflict(out string) bool {
	if strings.Contains(out, "CONFLICT") {
		return true
	}
	if strings.Contains(out, "Automatic merge failed") {
		return true
	}
	return false
}
