package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newNewCmd() *cobra.Command {
	var opts items.Options
	cmd := &cobra.Command{
		Use:   "new <type> <title>",
		Short: "Create a new item file",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runNew(args, cmd.OutOrStdout(), opts); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Priority, "priority", "", "P0|P1|P2|P3 (default from config or P2)")
	cmd.Flags().StringVar(&opts.Estimate, "estimate", "", "duration like 30m, 1h, 4h, 1d (default from config or 1h)")
	cmd.Flags().StringVar(&opts.Risk, "risk", "", "low|medium|high (default from config or low)")
	cmd.Flags().StringVar(&opts.Area, "area", "", "freeform area tag (default <fill-in>)")
	cmd.Flags().BoolVar(&opts.Ready, "ready", false, "create as immediately-claimable (skips the captured/inbox state)")
	return cmd
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

func runNew(args []string, stdout io.Writer, opts items.Options) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad new <type> \"<title>\"")
		return 2
	}
	typ := strings.ToLower(args[0])
	title := strings.Join(args[1:], " ")
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, "squad new: title required (got empty string)")
		return 4
	}
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

	// Flags > config > built-in default. items.NewWithOptions handles the
	// final fallback when an option is empty.
	if opts.Priority == "" {
		opts.Priority = cfg.Defaults.Priority
	}
	if opts.Estimate == "" {
		opts.Estimate = cfg.Defaults.Estimate
	}
	if opts.Risk == "" {
		opts.Risk = cfg.Defaults.Risk
	}
	if opts.Area == "" {
		opts.Area = cfg.Defaults.Area
	}

	agentID, _ := identity.AgentID()
	opts.CapturedBy = agentID

	squadDir := filepath.Join(root, ".squad")
	path, err := items.NewWithOptions(squadDir, prefix, title, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new item: %v\n", err)
		return 4
	}

	parsed, err := persistNewItem(root, path, agentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 4
	}

	if parsed.Status == "open" {
		fmt.Fprintf(stdout, "ready %s (immediately claimable)\n%s\n", parsed.ID, path)
	} else {
		fmt.Fprintf(stdout, "captured %s (run 'squad accept %s' when ready, or 'squad reject %s --reason ...')\n%s\n",
			parsed.ID, parsed.ID, parsed.ID, path)
	}
	return 0
}

func persistNewItem(root, path, agentID string) (items.Item, error) {
	db, err := store.OpenDefault()
	if err != nil {
		return items.Item{}, fmt.Errorf("persist new item: open store: %w", err)
	}
	defer db.Close()
	repoID, err := repo.IDFor(root)
	if err != nil {
		return items.Item{}, fmt.Errorf("persist new item: repo id: %w", err)
	}
	parsed, err := items.Parse(path)
	if err != nil {
		return items.Item{}, fmt.Errorf("persist new item: parse: %w", err)
	}
	ctx := context.Background()
	if err := items.Persist(ctx, db, repoID, parsed, false); err != nil {
		return items.Item{}, fmt.Errorf("persist new item: %w", err)
	}
	body := fmt.Sprintf("new %s in area %s — heads-up, you've been the top closer here recently",
		parsed.ID, parsed.Area)
	notifyAreaTopCloser(ctx, db, chat.New(db, repoID), repoID, agentID, parsed.Area, chat.ThreadGlobal, body)
	return parsed, nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
