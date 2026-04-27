package main

import (
	"context"
	"database/sql"
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

// RecaptureArgs is the input for Recapture. DB and RepoID are required;
// AgentID defaults to the session-derived identity when empty.
type RecaptureArgs struct {
	DB      *sql.DB `json:"-"`
	RepoID  string  `json:"repo_id"`
	AgentID string  `json:"agent_id"`
	ItemID  string  `json:"item_id"`
}

// RecaptureResult reports the outcome of a successful recapture.
type RecaptureResult struct {
	ItemID  string `json:"item_id"`
	AgentID string `json:"agent_id"`
}

// Recapture moves a needs-refinement item back to the inbox after the
// agent edited the body in response to reviewer feedback. The reviewer's
// `## Reviewer feedback` section is migrated into `## Refinement history`
// as a new round, the item status flips to `captured`, and the active
// claim is released. All bookkeeping is delegated to items.Recapture so
// the CLI and MCP paths cannot drift.
func Recapture(ctx context.Context, args RecaptureArgs) (*RecaptureResult, error) {
	if err := items.Recapture(ctx, args.DB, args.RepoID, args.ItemID, args.AgentID); err != nil {
		return nil, err
	}
	return &RecaptureResult{
		ItemID:  args.ItemID,
		AgentID: args.AgentID,
	}, nil
}

func newRecaptureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recapture <ID>",
		Short: "Send a needs-refinement item back to the inbox after editing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runRecapture(cmd.Context(), args[0], cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	return cmd
}

func runRecapture(ctx context.Context, id string, stdout, stderr io.Writer) int {
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

	agentID, err := identity.AgentID()
	if err != nil {
		fmt.Fprintf(stderr, "agent id: %v\n", err)
		return 4
	}

	_, err = Recapture(ctx, RecaptureArgs{
		DB:      db,
		RepoID:  repoID,
		AgentID: agentID,
		ItemID:  id,
	})
	switch {
	case err == nil:
		fmt.Fprintf(stdout, "recaptured %s\n", id)
		return 0
	case errors.Is(err, items.ErrItemNotFound):
		fmt.Fprintf(stderr, "%s: no such item\n", id)
		return 1
	case errors.Is(err, items.ErrClaimNotHeld):
		fmt.Fprintf(stderr, "%s: you don't hold a claim on this item — claim first\n", id)
		return 1
	case errors.Is(err, items.ErrWrongStatusForRecapture):
		fmt.Fprintf(stderr, "%s: item is not in needs-refinement\n", id)
		return 1
	default:
		fmt.Fprintf(stderr, "%s: %v\n", id, err)
		return 1
	}
}
