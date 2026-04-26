package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
	"github.com/zsiec/squad/internal/repo"
)

func newLearningListCmd() *cobra.Command {
	var area, state, kind string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List learning artifacts (filterable by --area, --state, --kind)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, _ := os.Getwd()
			root, err := repo.Discover(wd)
			if err != nil {
				return err
			}
			ls, err := learning.Walk(root)
			if err != nil {
				return err
			}
			for _, l := range ls {
				if area != "" && l.Area != area {
					continue
				}
				if state != "" && string(l.State) != state {
					continue
				}
				if kind != "" && string(l.Kind) != kind {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n",
					l.State, l.Kind, l.Area, l.Slug, l.Path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&area, "area", "", "filter by area")
	cmd.Flags().StringVar(&state, "state", "", "filter by state")
	cmd.Flags().StringVar(&kind, "kind", "", "filter by kind")
	return cmd
}
