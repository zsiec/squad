package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

var diffFenceRe = regexp.MustCompile("(?s)```diff\\n(.*?)```")

var (
	ErrProposalNotFound = errors.New("agents-md proposal not found")
	ErrApplyFailed      = errors.New("git apply failed")
)

// ApplyFailedError carries the git apply stderr so the cobra wrapper can
// echo it to the user; MCP callers can read .Stderr directly.
type ApplyFailedError struct {
	Stderr string
	Err    error
}

func (e *ApplyFailedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrApplyFailed.Error(), strings.TrimSpace(e.Stderr))
}

func (e *ApplyFailedError) Is(target error) bool { return target == ErrApplyFailed }
func (e *ApplyFailedError) Unwrap() error        { return e.Err }

type LearningAgentsMdApproveArgs struct {
	RepoRoot string `json:"repo_root"`
	ID       string `json:"id"`
}

type LearningAgentsMdApproveResult struct {
	AppliedPath string `json:"applied_path"`
}

func LearningAgentsMdApprove(_ context.Context, args LearningAgentsMdApproveArgs) (*LearningAgentsMdApproveResult, error) {
	proposed := filepath.Join(args.RepoRoot, ".squad", "learnings", "agents-md", "proposed", args.ID+".md")
	body, err := os.ReadFile(proposed)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrProposalNotFound, proposed)
		}
		return nil, fmt.Errorf("read proposal: %w", err)
	}
	diff := extractDiff(body)
	if diff == "" {
		return nil, fmt.Errorf("no diff fence found in proposal %s", proposed)
	}
	if err := gitApply(args.RepoRoot, diff); err != nil {
		return nil, err
	}
	applied := filepath.Join(args.RepoRoot, ".squad", "learnings", "agents-md", "applied", args.ID+".md")
	if err := os.MkdirAll(filepath.Dir(applied), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(applied, learning.RewriteState(body, learning.State("applied")), 0o644); err != nil {
		return nil, err
	}
	if err := os.Remove(proposed); err != nil {
		return nil, err
	}
	return &LearningAgentsMdApproveResult{AppliedPath: applied}, nil
}

func newLearningAgentsMdApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents-md-approve <id>",
		Short: "Apply a proposed AGENTS.md diff via `git apply`; on success, archive the proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			res, err := LearningAgentsMdApprove(cmd.Context(), LearningAgentsMdApproveArgs{
				RepoRoot: root, ID: args[0],
			})
			if err != nil {
				var af *ApplyFailedError
				if errors.As(err, &af) {
					_, _ = cmd.ErrOrStderr().Write([]byte(af.Stderr))
					return fmt.Errorf("git apply: %w", af.Err)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied: %s\n", res.AppliedPath)
			return nil
		},
	}
}

func extractDiff(body []byte) string {
	m := diffFenceRe.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func gitApply(repoRoot, diff string) error {
	c := exec.Command("git", "apply", "--whitespace=nowarn", "-")
	c.Dir = repoRoot
	c.Stdin = strings.NewReader(diff)
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	if err := c.Run(); err != nil {
		return &ApplyFailedError{Stderr: errBuf.String(), Err: err}
	}
	return nil
}
