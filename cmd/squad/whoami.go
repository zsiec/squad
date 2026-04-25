package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/store"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print the agent id this session resolves to",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := store.EnsureHome(); err != nil {
				return err
			}
			id, err := identity.AgentID("")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), id)
			return nil
		},
	}
}
