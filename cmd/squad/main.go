package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const versionString = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "squad",
		Short:         "Project-management framework for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newRegisterCmd())
	root.AddCommand(newWhoamiCmd())
	root.AddCommand(newNewCmd())
	root.AddCommand(newNextCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newClaimCmd())
	root.AddCommand(newReleaseCmd())
	root.AddCommand(newDoneCmd())
	root.AddCommand(newBlockedCmd())
	root.AddCommand(newForceReleaseCmd())
	root.AddCommand(newReassignCmd())
	root.AddCommand(newSayCmd())
	root.AddCommand(newAskCmd())
	root.AddCommand(newAnswerCmd())
	root.AddCommand(newChattyCmd(thinkingVerb))
	root.AddCommand(newChattyCmd(stuckVerb))
	root.AddCommand(newChattyCmd(milestoneVerb))
	root.AddCommand(newChattyCmd(fyiVerb))
	root.AddCommand(newKnockCmd())
	root.AddCommand(newHandoffCmd())
	root.AddCommand(newReviewRequestCmd())
	root.AddCommand(newProgressCmd())
	root.AddCommand(newTickCmd())
	root.AddCommand(newWhoCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the squad version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), versionString)
			return nil
		},
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
