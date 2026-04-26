package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newRejectCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "reject <id> [<id>...] --reason <text>",
		Short: "Reject captured items (delete + log)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runReject(cmd.Context(), args, reason, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "human-readable reason for rejection (required)")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

func runReject(ctx context.Context, ids []string, reason string, stdout, stderr io.Writer) int {
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
	squadDir := filepath.Join(root, ".squad")

	anyFailed := false
	for _, id := range ids {
		err := items.Reject(ctx, db, repoID, id, reason, agentID, squadDir)
		if err == nil {
			fmt.Fprintf(stdout, "rejected %s\n", id)
			continue
		}
		anyFailed = true
		if errors.Is(err, items.ErrItemClaimed) {
			fmt.Fprintf(stderr, "%s: claimed by another agent — force-release first\n", id)
			continue
		}
		if errors.Is(err, items.ErrReasonRequired) {
			fmt.Fprintf(stderr, "%s: %v\n", id, err)
			continue
		}
		fmt.Fprintf(stderr, "%s: %v\n", id, err)
	}
	if anyFailed {
		return 1
	}
	return 0
}
