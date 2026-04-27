// Package worktree provisions and tears down isolated git worktrees on
// behalf of a claim. Each claim that opts in gets a checkout under
// .squad/worktrees/<agent>-<item>/ on a dedicated squad/<item>-<agent>
// branch so concurrent agents can run tests without sharing pending edits.
package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrExists signals that Provision found a pre-existing worktree for the
// (itemID, agentID) pair. The path and branch returned alongside are valid
// — callers may treat re-entry as success when resuming a claim.
var ErrExists = errors.New("worktree: already provisioned")

// ErrNotGitRepo signals that repoRoot is not a git working tree. The
// shell-out helpers all front-load this check so a misconfigured caller
// gets a clear message instead of a cryptic git error.
var ErrNotGitRepo = errors.New("worktree: not a git repository")

// Worktree is one entry returned by List. HEAD is the short SHA at the
// time of the call; Branch is the symbolic ref name (e.g. "main",
// "squad/FEAT-005-agent-1f3f"), empty for detached HEADs.
type Worktree struct {
	Path   string
	HEAD   string
	Branch string
}

// BranchName is the deterministic branch name a (itemID, agentID) pair
// resolves to. Exposed so callers (Cleanup, doctor, tests) can recompute it
// without re-implementing the convention.
func BranchName(itemID, agentID string) string {
	return "squad/" + itemID + "-" + agentID
}

// Path is the deterministic worktree path under <repoRoot>/.squad/worktrees/.
// The agent-first ordering keeps a single agent's worktrees adjacent in `ls`
// output and matches the existing display convention in agents tables.
func Path(repoRoot, itemID, agentID string) string {
	return filepath.Join(repoRoot, ".squad", "worktrees", agentID+"-"+itemID)
}

// Provision creates a new worktree for the (itemID, agentID) pair on a
// fresh squad/<item>-<agent> branch rooted at baseBranch. If a worktree at
// the conventional path already exists it returns the existing path/branch
// and ErrExists — callers can treat that as a no-op resume. Any other
// failure (dirty base, branch collision, missing baseBranch) is surfaced
// verbatim from git's stderr so the user sees the same message they'd get
// running the command by hand.
func Provision(repoRoot, baseBranch, itemID, agentID string) (string, string, error) {
	if !isGitRepo(repoRoot) {
		return "", "", ErrNotGitRepo
	}
	branch := BranchName(itemID, agentID)
	path := Path(repoRoot, itemID, agentID)

	if existing, err := findWorktree(repoRoot, path); err == nil && existing.Path != "" {
		return existing.Path, branch, ErrExists
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", fmt.Errorf("worktree: create parent: %w", err)
	}
	args := []string{"worktree", "add", "-b", branch, path}
	if baseBranch != "" {
		args = append(args, baseBranch)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	// Persist the base so Cleanup can decide whether to delete the branch
	// without relying on a literal "main"/"master" guess. Without this, a
	// repo whose default branch is e.g. "develop" leaves every squad/ branch
	// behind on close-out.
	if baseBranch != "" {
		setCfg := exec.Command("git", "config", "branch."+branch+".squadBase", baseBranch)
		setCfg.Dir = repoRoot
		_ = setCfg.Run()
	}
	abs, _ := filepath.Abs(path)
	return abs, branch, nil
}

// Cleanup removes the worktree at path and, if the matching squad/ branch
// has no commits ahead of its base, deletes the branch. A branch with new
// commits is left in place so the user (or a follow-up PR) owns the merge.
// A missing or already-removed worktree is treated as success.
func Cleanup(repoRoot, path string) error {
	if !isGitRepo(repoRoot) {
		return ErrNotGitRepo
	}

	branch, base := branchAndBaseFor(repoRoot, path)

	cmd := exec.Command("git", "worktree", "remove", "--force", path)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "is not a working tree") || strings.Contains(msg, "No such file") {
			prune := exec.Command("git", "worktree", "prune")
			prune.Dir = repoRoot
			_ = prune.Run()
		} else {
			return fmt.Errorf("git worktree remove %s: %w: %s", path, err, msg)
		}
	}

	if branch != "" && strings.HasPrefix(branch, "squad/") {
		if !branchHasNewCommits(repoRoot, branch, base) {
			del := exec.Command("git", "branch", "-D", branch)
			del.Dir = repoRoot
			_ = del.Run()
		}
	}
	return nil
}

// List returns one entry per registered worktree, including the main one.
// Callers (squad doctor) compare returned paths against the active claims
// to surface orphaned squad/ worktrees.
func List(repoRoot string) ([]Worktree, error) {
	if !isGitRepo(repoRoot) {
		return nil, ErrNotGitRepo
	}
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parseWorktreeList(string(out)), nil
}

func parseWorktreeList(s string) []Worktree {
	var out []Worktree
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			out = append(out, cur)
		}
		cur = Worktree{}
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		k, v, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch k {
		case "worktree":
			cur.Path = v
		case "HEAD":
			cur.HEAD = v
		case "branch":
			cur.Branch = strings.TrimPrefix(v, "refs/heads/")
		}
	}
	flush()
	return out
}

func findWorktree(repoRoot, path string) (Worktree, error) {
	list, err := List(repoRoot)
	if err != nil {
		return Worktree{}, err
	}
	abs, _ := filepath.Abs(path)
	for _, w := range list {
		if pathsEqual(w.Path, path) || pathsEqual(w.Path, abs) {
			return w, nil
		}
	}
	return Worktree{}, nil
}

func branchAndBaseFor(repoRoot, path string) (branch, base string) {
	w, err := findWorktree(repoRoot, path)
	if err != nil {
		return "", ""
	}
	branch = w.Branch
	if !strings.HasPrefix(branch, "squad/") {
		return branch, ""
	}
	cmd := exec.Command("git", "config", "--get", "branch."+branch+".squadBase")
	cmd.Dir = repoRoot
	if out, err := cmd.Output(); err == nil {
		base = strings.TrimSpace(string(out))
	}
	if base == "" {
		base = detectMainBranch(repoRoot)
	}
	return branch, base
}

func branchHasNewCommits(repoRoot, branch, base string) bool {
	if base == "" {
		return true
	}
	cmd := exec.Command("git", "rev-list", "--count", branch, "^"+base)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return true
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return true
	}
	return n > 0
}

func detectMainBranch(repoRoot string) string {
	for _, candidate := range []string{"main", "master"} {
		c := exec.Command("git", "rev-parse", "--verify", "--quiet", candidate)
		c.Dir = repoRoot
		if err := c.Run(); err == nil {
			return candidate
		}
	}
	return ""
}

func isGitRepo(repoRoot string) bool {
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err == nil {
		return true
	}
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}

func pathsEqual(a, b string) bool {
	if a == b {
		return true
	}
	aa, errA := filepath.EvalSymlinks(a)
	bb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return false
	}
	return aa == bb
}
