package prmark

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Commit is one entry returned by ResolveCommits.
type Commit struct {
	Sha     string `json:"sha"`
	Subject string `json:"subject"`
}

// CommitQuery scopes a git-log invocation to one branch, an optional
// time window, and (optionally) a touched-files pathspec.
type CommitQuery struct {
	RepoRoot     string
	Branch       string
	TouchedFiles []string
	Since        time.Time
	Until        time.Time
	Limit        int
}

// safeBranchRE constrains branch names to git-portable characters that
// are also safe to pass as a positional argument to `git log`. Git's
// own check-ref-format is broader, but we are deliberately stricter
// here because the branch flows in from squad's pending-prs.json (and
// thus ultimately from `squad pr-link`-time user input).
var safeBranchRE = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// ResolveCommits shells out to `git log` to produce the commits on
// q.Branch that touched any file in q.TouchedFiles, capped at q.Limit.
// When q.Since / q.Until are non-zero they bound the commit window.
//
// Branch and pathspec are sandboxed: branch must match safeBranchRE,
// must not contain ".." (path-traversal escape), and must not start
// with "-" (would be parsed as a flag). Arguments are passed via the
// exec.Command arg slice — no shell interpretation.
func ResolveCommits(ctx context.Context, q CommitQuery) ([]Commit, error) {
	if q.Branch == "" {
		return nil, fmt.Errorf("resolve commits: branch required")
	}
	if strings.HasPrefix(q.Branch, "-") {
		return nil, fmt.Errorf("resolve commits: branch %q starts with '-'", q.Branch)
	}
	if strings.Contains(q.Branch, "..") {
		return nil, fmt.Errorf("resolve commits: branch %q contains '..'", q.Branch)
	}
	if !safeBranchRE.MatchString(q.Branch) {
		return nil, fmt.Errorf("resolve commits: branch %q contains disallowed characters", q.Branch)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	args := []string{"-C", q.RepoRoot, "log", q.Branch, "-n", strconv.Itoa(limit), "--no-color", "--format=%H%x09%s"}
	if !q.Since.IsZero() {
		args = append(args, "--since="+q.Since.Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		args = append(args, "--until="+q.Until.Format(time.RFC3339))
	}
	if len(q.TouchedFiles) > 0 {
		args = append(args, "--")
		args = append(args, q.TouchedFiles...)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	var out []Commit
	for _, line := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		sha, subject, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		out = append(out, Commit{Sha: sha, Subject: subject})
	}
	return out, nil
}
