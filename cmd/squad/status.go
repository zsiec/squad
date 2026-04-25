package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show in-progress / ready / blocked counts for this repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runStatus(args, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runStatus(_ []string, stdout io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 4
	}
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo: %v\n", err)
		return 4
	}
	squadDir := filepath.Join(root, ".squad")
	w, err := items.Walk(squadDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk items: %v\n", err)
		return 4
	}

	if db, derr := store.OpenDefault(); derr == nil {
		defer db.Close()
		if repoID, rerr := repo.IDFor(root); rerr == nil {
			_ = items.Mirror(context.Background(), db, repoID, w)
		}
	}

	c := items.Counts(w, time.Now().UTC())
	fmt.Fprintf(stdout, "in_progress: %d\nready: %d\nblocked: %d\n",
		c.InProgress, c.Ready, c.Blocked)
	return 0
}
