package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/repo"
)

func newEpicNewCmd() *cobra.Command {
	var spec string
	cmd := &cobra.Command{
		Use:   "epic-new <name>",
		Short: "Create an epic scaffold under .squad/epics/<name>.md",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runEpicNew(args, spec, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&spec, "spec", "", "spec slug this epic belongs to (required)")
	_ = cmd.MarkFlagRequired("spec")
	return cmd
}

const epicStub = `---
spec: %s
status: open
parallelism: |
  Filled in by 'squad analyze %s' — do not hand-edit.
---

## Goal
`

func runEpicNew(args []string, spec string, stdout io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: squad epic-new <name> --spec <name>")
		return 2
	}
	name := strings.TrimSpace(args[0])
	if !slugSafe.MatchString(name) || !slugSafe.MatchString(spec) {
		fmt.Fprintf(os.Stderr, "names must be kebab-case (got name=%q spec=%q)\n", name, spec)
		return 4
	}
	wd, _ := os.Getwd()
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	specPath := filepath.Join(root, ".squad", "specs", spec+".md")
	if _, err := os.Stat(specPath); err != nil {
		fmt.Fprintf(os.Stderr, "spec %s does not exist\n", specPath)
		return 4
	}
	dir := filepath.Join(root, ".squad", "epics")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	path := filepath.Join(dir, name+".md")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "epic %s already exists\n", path)
		return 4
	}
	if err := os.WriteFile(path, []byte(fmt.Sprintf(epicStub, spec, name)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Fprintln(stdout, path)
	return 0
}
