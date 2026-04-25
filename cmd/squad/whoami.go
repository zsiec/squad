package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print the agent id this session resolves to",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implemented yet")
		},
	}
}
