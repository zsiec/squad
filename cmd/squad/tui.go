package main

import (
	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/tui"
)

func newTUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "k9s-style terminal UI for squad",
		Long: `k9s-style terminal UI for squad.

Launches a terminal UI that connects to a running squad serve daemon and
provides interactive views of items, agents, chat, specs, epics, evidence,
learnings, and statistics. Auto-installs the squad serve daemon on first
launch (launchd on darwin, systemctl --user on linux).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(cmd.Context())
		},
	}
	cmd.Flags().String("serve-url", "", "override serve URL (default: auto-detect)")
	cmd.Flags().String("token-file", "", "override token file (default: ~/.squad/token)")
	cmd.Flags().Bool("embedded", false, "run serve in-process (no daemon install)")
	return cmd
}
