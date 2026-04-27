package main

import "github.com/spf13/cobra"

func newLearningCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learning",
		Short: "Propose, list, approve, and reject learning artifacts; manage AGENTS.md change proposals.",
		Long: `Learning artifacts are gotchas, patterns, and dead-ends discovered while working in this repo. Agents propose; humans approve. Approved learnings auto-load into future sessions whose work touches matching paths.

AGENTS.md follows a stricter gate: agents propose changes via a unified diff; only humans apply them.`,
	}
	cmd.AddCommand(newLearningProposeCmd())
	cmd.AddCommand(newLearningQuickCmd())
	cmd.AddCommand(newLearningListCmd())
	cmd.AddCommand(newLearningApproveCmd())
	cmd.AddCommand(newLearningRejectCmd())
	cmd.AddCommand(newLearningAgentsMdSuggestCmd())
	cmd.AddCommand(newLearningAgentsMdApproveCmd())
	cmd.AddCommand(newLearningAgentsMdRejectCmd())
	cmd.AddCommand(newLearningTrivialityCheckCmd())
	return cmd
}
