package server

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/repo"
)

type agentDiffFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Hunks  string `json:"hunks"`
}

type agentDiffResponse struct {
	AgentID     string          `json:"agent_id"`
	MergeTarget string          `json:"merge_target"`
	Worktree    string          `json:"worktree,omitempty"`
	Files       []agentDiffFile `json:"files"`
}

// handleAgentDiff returns the worktree-vs-merge-target diff for the given
// agent's most recent active claim. The SPA polls this endpoint while the
// diff panel is open (~20s cadence) so the watcher can follow the agent's
// progress without leaving the browser.
func (s *Server) handleAgentDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out := agentDiffResponse{AgentID: id, Files: []agentDiffFile{}}

	var worktree string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(worktree, '') FROM claims
		 WHERE agent_id = ? AND COALESCE(worktree, '') != ''
		 ORDER BY claimed_at DESC LIMIT 1`,
		id,
	).Scan(&worktree)
	if errors.Is(err, sql.ErrNoRows) {
		out.MergeTarget = mergeTargetForRepo(s.cfg.LearningsRoot)
		writeJSON(w, http.StatusOK, out)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Worktree = worktree
	out.MergeTarget = mergeTargetForRepo(s.cfg.LearningsRoot)

	files, err := collectWorktreeDiff(worktree, out.MergeTarget)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Files = files
	writeJSON(w, http.StatusOK, out)
}

// mergeTargetForRepo returns the configured merge target branch for the
// repo at root, falling back to the package default. Mirrors the policy
// `squad done`'s worktree-fold uses so the diff panel and the close path
// agree on what "main" means.
func mergeTargetForRepo(root string) string {
	if root == "" {
		return config.DefaultMergeTargetBranch
	}
	discovered, err := repo.Discover(root)
	if err != nil {
		return config.DefaultMergeTargetBranch
	}
	cfg, err := config.Load(discovered)
	if err != nil {
		return config.DefaultMergeTargetBranch
	}
	if t := strings.TrimSpace(cfg.Agent.MergeTargetBranch); t != "" {
		return t
	}
	return config.DefaultMergeTargetBranch
}

// collectWorktreeDiff runs `git diff <mergeTarget>` from the worktree
// (covering committed branch divergence + unstaged + untracked deltas)
// and parses the unified-diff output into per-file structured rows.
// Untracked files are surfaced via `git ls-files --others --exclude-standard`
// and rendered as "added" with their full body as a synthetic hunk; git
// diff against a tree omits them by default.
func collectWorktreeDiff(worktree, mergeTarget string) ([]agentDiffFile, error) {
	tracked, err := runGitDiff(worktree, mergeTarget)
	if err != nil {
		return nil, err
	}
	untracked, err := collectUntracked(worktree)
	if err != nil {
		return nil, err
	}
	return append(tracked, untracked...), nil
}

func runGitDiff(worktree, mergeTarget string) ([]agentDiffFile, error) {
	cmd := exec.Command("git", "diff", "--no-color", mergeTarget)
	cmd.Dir = worktree
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, errors.New("git diff: " + string(exitErr.Stderr))
		}
		return nil, err
	}
	return parseUnifiedDiff(string(out)), nil
}

// parseUnifiedDiff splits unified-diff output into per-file blocks
// keyed by the post-image path with a status derived from the file
// header. Pure stdlib — pulling in a diff library is overkill for what
// the SPA needs (display-only).
func parseUnifiedDiff(raw string) []agentDiffFile {
	if raw == "" {
		return nil
	}
	var (
		out     []agentDiffFile
		current *agentDiffFile
		header  []string
	)
	flush := func() {
		if current == nil {
			return
		}
		current.Hunks = strings.Join(header, "\n")
		out = append(out, *current)
		current = nil
		header = nil
	}
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			path := extractPostPathFromGitHeader(line)
			current = &agentDiffFile{Path: path, Status: "modified"}
			header = []string{line}
		case current != nil && strings.HasPrefix(line, "new file mode"):
			current.Status = "added"
			header = append(header, line)
		case current != nil && strings.HasPrefix(line, "deleted file mode"):
			current.Status = "deleted"
			header = append(header, line)
		case current != nil:
			header = append(header, line)
		}
	}
	flush()
	return out
}

// extractPostPathFromGitHeader pulls the post-image path out of a
// `diff --git a/<old> b/<new>` line. Returns the new-side path, which
// matches what users expect (added/modified/renamed paths).
func extractPostPathFromGitHeader(line string) string {
	const prefix = "diff --git "
	rest := strings.TrimPrefix(line, prefix)
	idx := strings.Index(rest, " b/")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(rest[idx+len(" b/"):])
}

// collectUntracked surfaces files present in the worktree but not yet
// committed and not tracked by git. They count as "added" for the
// dashboard's purposes — the watcher wants to see what the agent has
// brought into existence, not just what it has staged.
func collectUntracked(worktree string) ([]agentDiffFile, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = worktree
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, errors.New("git ls-files: " + string(exitErr.Stderr))
		}
		return nil, err
	}
	var files []agentDiffFile
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" {
			continue
		}
		body, err := readFileBounded(worktree, path)
		if err != nil {
			continue
		}
		files = append(files, agentDiffFile{
			Path:   path,
			Status: "added",
			Hunks:  "diff --git a/" + path + " b/" + path + "\nnew file (untracked)\n--- /dev/null\n+++ b/" + path + "\n" + prefixLines(body, "+"),
		})
	}
	return files, nil
}

// readFileBounded reads up to 256KiB from a file inside worktree.
// Larger payloads are truncated so a runaway agent committing a multi-MB
// blob doesn't blow up the JSON response. The SPA shows the truncation
// notice and the user can drop to a terminal for the full file.
func readFileBounded(worktree, path string) (string, error) {
	const limit = 256 * 1024
	f, err := os.Open(filepath.Join(worktree, path))
	if err != nil {
		return "", err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return "", err
	}
	out := string(body)
	if len(body) >= limit {
		out += "\n[truncated at 256KiB — view in terminal for full content]\n"
	}
	return out, nil
}

func prefixLines(body, prefix string) string {
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
