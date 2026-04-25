package main

import (
	"errors"

	"github.com/spf13/cobra"
)

var errPRNotImplemented = errors.New("not implemented")

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

func runPRLink(cmd *cobra.Command, args []string) error  { return errPRNotImplemented }
func runPRClose(cmd *cobra.Command, args []string) error { return errPRNotImplemented }
