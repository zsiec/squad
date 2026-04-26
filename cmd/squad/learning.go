package main

import "github.com/spf13/cobra"

func newLearningCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "learning", Short: "Learning artifacts"}
	cmd.AddCommand(newLearningProposeCmd())
	cmd.AddCommand(newLearningListCmd())
	cmd.AddCommand(newLearningApproveCmd())
	cmd.AddCommand(newLearningRejectCmd())
	cmd.AddCommand(newLearningAgentsMdSuggestCmd())
	cmd.AddCommand(newLearningAgentsMdApproveCmd())
	cmd.AddCommand(newLearningAgentsMdRejectCmd())
	return cmd
}
