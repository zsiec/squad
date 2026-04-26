package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
	"github.com/zsiec/squad/internal/repo"
)

var diffFenceRe = regexp.MustCompile("(?s)```diff\\n(.*?)```")

func newLearningAgentsMdApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agents-md-approve <id>",
		Short: "Apply a proposed AGENTS.md diff via `git apply`; on success, archive the proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			wd, _ := os.Getwd()
			root, err := repo.Discover(wd)
			if err != nil {
				return err
			}
			proposed := filepath.Join(root, ".squad", "learnings", "agents-md", "proposed", id+".md")
			body, err := os.ReadFile(proposed)
			if err != nil {
				return fmt.Errorf("read proposal: %w", err)
			}
			diff := extractDiff(body)
			if diff == "" {
				return fmt.Errorf("no diff fence found in proposal %s", proposed)
			}
			if err := gitApply(root, diff, cmd.ErrOrStderr()); err != nil {
				return fmt.Errorf("git apply: %w", err)
			}
			applied := filepath.Join(root, ".squad", "learnings", "agents-md", "applied", id+".md")
			if err := os.MkdirAll(filepath.Dir(applied), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(applied, learning.RewriteState(body, learning.State("applied")), 0o644); err != nil {
				return err
			}
			if err := os.Remove(proposed); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied: %s\n", applied)
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

func gitApply(repoRoot, diff string, stderr io.Writer) error {
	c := exec.Command("git", "apply", "--whitespace=nowarn", "-")
	c.Dir = repoRoot
	c.Stdin = strings.NewReader(diff)
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	if err := c.Run(); err != nil {
		_, _ = stderr.Write(errBuf.Bytes())
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(errBuf.String()))
	}
	return nil
}
