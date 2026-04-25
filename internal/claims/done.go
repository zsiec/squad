package claims

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zsiec/squad/internal/items"
)

type DoneOpts struct {
	Summary  string
	ItemPath string
	DoneDir  string
}

func (s *Store) Done(ctx context.Context, itemID, agentID string, opts DoneOpts) error {
	now := s.nowUnix()

	dbErr := s.withTx(ctx, func(tx *sql.Tx) error {
		body := "done " + itemID
		if opts.Summary != "" {
			body += ": " + opts.Summary
		}
		if err := postSystemMessage(ctx, tx, s.repoID, now, agentID, "global", "done", body, nil, "normal"); err != nil {
			return err
		}
		if err := postSystemMessage(ctx, tx, s.repoID, now, agentID, itemID, "done", body, nil, "normal"); err != nil {
			return err
		}
		return s.releaseInTx(ctx, tx, itemID, agentID, "done")
	})
	if dbErr != nil {
		return dbErr
	}

	if opts.ItemPath == "" || opts.DoneDir == "" {
		return nil
	}
	if err := items.RewriteStatus(opts.ItemPath, "done", s.now()); err != nil {
		return fmt.Errorf("rewrite item status: %w", err)
	}
	if _, err := items.MoveToDone(opts.ItemPath, opts.DoneDir); err != nil {
		return fmt.Errorf("move to done dir: %w", err)
	}
	return nil
}
