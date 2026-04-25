package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/touch"
)

func newTouchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "touch <ITEM-ID> <path>...",
		Short: "Declare files you are editing so peers see the overlap",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			itemID, paths := args[0], args[1:]
			tr := touch.New(bc.db, bc.repoID)
			for _, p := range paths {
				conflicts, err := tr.Add(ctx, bc.agentID, itemID, p)
				if err != nil {
					return err
				}
				if len(conflicts) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"WARNING: %s also touched by: %s\n", p, strings.Join(conflicts, ", "))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "touched %s\n", p)
			}
			return nil
		},
	}
}

func newUntouchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untouch [path]...",
		Short: "Release file touches; no paths releases all touches for this agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			tr := touch.New(bc.db, bc.repoID)
			if len(args) == 0 {
				n, err := tr.ReleaseAll(ctx, bc.agentID)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "released %d touch(es)\n", n)
				return nil
			}
			for _, p := range args {
				if err := tr.Release(ctx, bc.agentID, p); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "released %s\n", p)
			}
			return nil
		},
	}
}
