package claims

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zsiec/squad/internal/items"
)

type BlockedOpts struct {
	Reason   string
	ItemPath string
}

func (s *Store) Blocked(ctx context.Context, itemID, agentID string, opts BlockedOpts) error {
	now := s.nowUnix()

	dbErr := s.withTx(ctx, func(tx *sql.Tx) error {
		body := "blocked on " + itemID
		if opts.Reason != "" {
			body += ": " + opts.Reason
		}
		if err := postSystemMessage(ctx, tx, s.repoID, now, agentID, "global", "blocked", body, nil, "normal"); err != nil {
			return err
		}
		if err := postSystemMessage(ctx, tx, s.repoID, now, agentID, itemID, "blocked", body, nil, "normal"); err != nil {
			return err
		}
		return s.releaseInTx(ctx, tx, itemID, agentID, "blocked")
	})
	if dbErr != nil {
		return dbErr
	}

	if opts.ItemPath == "" {
		return nil
	}
	if err := items.RewriteStatus(opts.ItemPath, "blocked", s.now()); err != nil {
		return fmt.Errorf("rewrite item status: %w", err)
	}
	if err := items.EnsureBlockerSection(opts.ItemPath, opts.Reason); err != nil {
		return fmt.Errorf("ensure blocker section: %w", err)
	}
	parsed, err := items.Parse(opts.ItemPath)
	if err != nil {
		return fmt.Errorf("parse blocked item for persist: %w", err)
	}
	if err := items.Persist(ctx, s.db, s.repoID, parsed, false); err != nil {
		return fmt.Errorf("persist items row: %w", err)
	}
	return nil
}
