package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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
	squadDir, code := discoverSquadDir()
	if code != 0 {
		return code
	}
	specPath := filepath.Join(squadDir, "specs", spec+".md")
	if _, err := os.Stat(specPath); err != nil {
		fmt.Fprintf(os.Stderr, "spec %s does not exist\n", specPath)
		return 4
	}
	body := fmt.Sprintf(epicStub, spec, name)
	_, code = writeScaffold(stdout, squadDir, "epics", name, body, "epic", true)
	return code
}
