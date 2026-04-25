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

func newNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "List ready items in priority order",
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runNext(args, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runNext(_ []string, stdout io.Writer) int {
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

	ready := items.Ready(w, time.Now().UTC())
	if len(ready) == 0 {
		fmt.Fprintln(os.Stderr, "no ready items")
		return 1
	}
	for _, it := range ready {
		fmt.Fprintf(stdout, "%-12s %-3s %-8s %s\n", it.ID, it.Priority, it.Estimate, it.Title)
	}
	return 0
}
