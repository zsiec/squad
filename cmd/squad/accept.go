package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newAcceptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "accept <id> [<id>...]",
		Short: "Promote captured items to open after Definition of Ready passes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runAccept(cmd.Context(), args, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runAccept(ctx context.Context, ids []string, stdout, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "getwd: %v\n", err)
		return 4
	}
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(stderr, "find repo: %v\n", err)
		return 4
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		fmt.Fprintf(stderr, "repo id: %v\n", err)
		return 4
	}
	db, err := store.OpenDefault()
	if err != nil {
		fmt.Fprintf(stderr, "open store: %v\n", err)
		return 4
	}
	defer db.Close()
	agentID, _ := identity.AgentID()

	anyFailed := false
	for _, id := range ids {
		err := items.Promote(ctx, db, repoID, id, agentID)
		if err == nil {
			fmt.Fprintf(stdout, "accepted %s\n", id)
			continue
		}
		anyFailed = true
		var dorErr *items.DoRError
		if errors.As(err, &dorErr) {
			fmt.Fprintf(stderr, "skipped %s — definition of ready failed:\n", id)
			for _, v := range dorErr.Violations {
				fmt.Fprintf(stderr, "  - [%s] %s\n", v.Rule, v.Message)
			}
			continue
		}
		fmt.Fprintf(stderr, "%s: %v\n", id, err)
	}
	if anyFailed {
		return 1
	}
	return 0
}
