package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newRegisterCmd() *cobra.Command {
	var (
		asFlag      string
		nameFlag    string
		noRepoCheck bool
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this agent in the squad global database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet")
		},
	}
	cmd.Flags().StringVar(&asFlag, "as", "", "agent id override (persisted per-session)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "display name (defaults to agent id)")
	cmd.Flags().BoolVar(&noRepoCheck, "no-repo-check", false, "internal — `init` will use this; users normally don't call register directly")
	return cmd
}
