package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newGoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go",
		Short: "Onboard or resume in one step (register, claim, mailbox flush)",
		Long: `go is the single-command entry point for an agent session.

It does whatever is needed to reach "claim held, AC loaded, mailbox
drained": init the workspace if .squad/ is absent, register the agent
if not already registered, find the top ready item, claim it, print
its acceptance criteria, and flush any pending chat into stdout.

Idempotent — running it twice does not claim two items. If a claim is
already held, go resumes that claim, re-prints its AC, and flushes
the mailbox.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGo(cmd)
		},
	}
	return cmd
}

func runGo(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := ensureSquadInit(wd, out); err != nil {
		return err
	}
	return nil
}

func ensureSquadInit(wd string, out io.Writer) error {
	if _, err := os.Stat(filepath.Join(wd, ".squad", "config.yaml")); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintln(out, "no .squad/ found — running squad init --yes")
	return runInit(&cobra.Command{}, initOptions{Yes: true, Dir: wd})
}
