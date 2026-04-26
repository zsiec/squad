package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

type LearningListArgs struct {
	RepoRoot string `json:"repo_root"`
	Area     string `json:"area,omitempty"`
	State    string `json:"state,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type LearningListResult struct {
	Items []*learning.Learning `json:"items"`
	Count int                  `json:"count"`
}

func LearningList(_ context.Context, args LearningListArgs) (*LearningListResult, error) {
	ls, err := learning.Walk(args.RepoRoot)
	if err != nil {
		return nil, err
	}
	out := make([]*learning.Learning, 0, len(ls))
	for i := range ls {
		l := ls[i]
		if args.Area != "" && l.Area != args.Area {
			continue
		}
		if args.State != "" && string(l.State) != args.State {
			continue
		}
		if args.Kind != "" && string(l.Kind) != args.Kind {
			continue
		}
		out = append(out, &l)
	}
	return &LearningListResult{Items: out, Count: len(out)}, nil
}

func newLearningListCmd() *cobra.Command {
	var area, state, kind string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List learning artifacts (filterable by --area, --state, --kind)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			res, err := LearningList(cmd.Context(), LearningListArgs{
				RepoRoot: root, Area: area, State: state, Kind: kind,
			})
			if err != nil {
				return err
			}
			for _, l := range res.Items {
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
