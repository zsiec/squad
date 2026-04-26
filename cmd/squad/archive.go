package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/store"
)

// ArchiveArgs is the input for Archive. Before is the cutoff: messages
// older than (now - Before) are rolled into the per-month archive DB.
type ArchiveArgs struct {
	DB     *sql.DB
	RepoID string
	Before time.Duration
}

// ArchiveResult reports the result of one archive run.
type ArchiveResult struct {
	MessagesArchived int    `json:"messages_archived"`
	ArchivePath      string `json:"archive_path"`
}

// Archive rolls chat messages older than args.Before into a per-month
// archive DB under ~/.squad/archive/.
func Archive(ctx context.Context, args ArchiveArgs) (*ArchiveResult, error) {
	if args.Before <= 0 {
		return nil, fmt.Errorf("archive: before must be positive")
	}
	home, err := store.Home()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "archive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-args.Before).Unix()
	n, path, err := hygiene.Archive(ctx, args.DB, args.RepoID, dir, cutoff)
	if err != nil {
		return nil, err
	}
	return &ArchiveResult{MessagesArchived: n, ArchivePath: path}, nil
}

func newArchiveCmd() *cobra.Command {
	var before string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Roll old chat messages into a per-month archive DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			dur, err := parseHumanDuration(before)
			if err != nil {
				return fmt.Errorf("invalid --before: %w", err)
			}
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			res, err := Archive(ctx, ArchiveArgs{
				DB: bc.db, RepoID: bc.repoID, Before: dur,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "archived %d message(s) to %s\n",
				res.MessagesArchived, res.ArchivePath)
			return nil
		},
	}
	cmd.Flags().StringVar(&before, "before", "30d", "cut-off duration (e.g. 30d, 720h)")
	return cmd
}

func parseHumanDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	var days int
	if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("could not parse %q", s)
}
