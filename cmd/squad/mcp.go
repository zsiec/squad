package main

import (
	"context"
	"database/sql"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/mcp"
	"github.com/zsiec/squad/internal/notify"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run an MCP server over stdio (Claude Code transport).",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, repoID, repoRoot, closeFn, err := openMCPContext()
			if err != nil {
				return err
			}
			defer closeFn()
			return runMCP(cmd.Context(), db, repoID, repoRoot, os.Stdin, os.Stdout)
		},
	}
}

// openMCPContext opens the global squad store and best-effort discovers the
// caller's repo. MCP must start even when no repo is found (claude-installed
// in a non-repo dir): missing repo surfaces as repoID="" and repoRoot="" so
// per-repo handlers can return a structured error rather than crashing init.
func openMCPContext() (*sql.DB, string, string, func(), error) {
	if err := store.EnsureHome(); err != nil {
		return nil, "", "", func() {}, err
	}
	db, err := store.OpenDefault()
	if err != nil {
		return nil, "", "", func() {}, err
	}
	closeFn := func() { _ = db.Close() }

	repoID := ""
	repoRoot := ""
	if wd, err := os.Getwd(); err == nil {
		if root, err := repo.Discover(wd); err == nil {
			repoRoot = root
			if id, err := repo.IDFor(root); err == nil {
				repoID = id
			}
		}
	}
	return db, repoID, repoRoot, closeFn, nil
}

func runMCP(ctx context.Context, db *sql.DB, repoID, repoRoot string, in io.Reader, out io.Writer) error {
	srv := mcp.NewServer(mcp.ServerInfo{Name: "squad", Version: versionString})
	registerTools(srv, db, repoID, repoRoot)
	return srv.Serve(ctx, in, out)
}

// newChatService mirrors bootClaimContext's chat wiring without bringing in
// the full claim-context machinery. MCP handlers that post to chat get the
// same notify-wake side-effect the CLI uses.
func newChatService(db *sql.DB, repoID string) *chat.Chat {
	c := chat.New(db, repoID)
	registry := notify.NewRegistry(db)
	c.SetNotifier(func(ctx context.Context, repoID string) {
		_ = notify.Wake(ctx, registry, repoID, 100*time.Millisecond)
	})
	return c
}
