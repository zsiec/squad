package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <type> <title>",
		Short: "Create a new item file",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runNew(args, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

var typeToPrefix = map[string]string{
	"bug":       "BUG",
	"feature":   "FEAT",
	"feat":      "FEAT",
	"task":      "TASK",
	"chore":     "CHORE",
	"tech-debt": "DEBT",
	"debt":      "DEBT",
	"bet":       "BET",
}

func runNew(args []string, stdout io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad new <type> \"<title>\"")
		return 2
	}
	typ := strings.ToLower(args[0])
	title := strings.Join(args[1:], " ")
	prefix, ok := typeToPrefix[typ]
	if !ok {
		prefix = strings.ToUpper(typ)
	}

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
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 4
	}
	if !containsString(cfg.IDPrefixes, prefix) {
		fmt.Fprintf(os.Stderr, "type %q maps to prefix %q which is not in id_prefixes %v\n",
			typ, prefix, cfg.IDPrefixes)
		return 4
	}
	squadDir := filepath.Join(root, ".squad")
	path, err := items.New(squadDir, prefix, title)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new item: %v\n", err)
		return 4
	}

	if db, derr := store.OpenDefault(); derr == nil {
		defer db.Close()
		if repoID, rerr := repo.IDFor(root); rerr == nil {
			if w, werr := items.Walk(squadDir); werr == nil {
				_ = items.Mirror(context.Background(), db, repoID, w)
			}
		}
	}

	fmt.Fprintln(stdout, path)
	return 0
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
