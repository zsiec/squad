package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/learning"
)

func newLearningTrivialityCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "triviality-check",
		Hidden: true,
		Short:  "Read git diff --numstat from stdin; print 'trivial' or 'non-trivial'.",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in := cmd.InOrStdin()
			if in == nil {
				in = os.Stdin
			}
			data, err := io.ReadAll(in)
			if err != nil {
				return err
			}
			if learning.NonTrivialDiff(string(data)) {
				fmt.Fprintln(cmd.OutOrStdout(), "non-trivial")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "trivial")
			}
			return nil
		},
	}
}
