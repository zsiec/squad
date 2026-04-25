package claims

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/zsiec/squad/internal/items"
)

type DoneOpts struct {
	Summary  string
	ItemPath string
	DoneDir  string
}

// Done finalises an item: rewrites its frontmatter to status: done, moves
// it into doneDir, and atomically posts the system messages + releases the
// claim in a single DB transaction.
//
// Ordering matters. The DB transaction runs LAST so a file-rewrite or move
// failure leaves no DB residue. If the DB commit itself fails, we roll
// back the file move (best effort) so the user can retry cleanly. The
// previous order (DB first, files second) hit QA r6-H #3: a partial file
// failure left the claim released but the file untouched, with no
// recovery path beyond manual intervention.
func (s *Store) Done(ctx context.Context, itemID, agentID string, opts DoneOpts) error {
	now := s.nowUnix()

	// File ops first, but only if the caller supplied paths. Tests that
	// don't care about the filesystem keep the original DB-only path.
	var movedFrom, movedTo string
	if opts.ItemPath != "" && opts.DoneDir != "" {
		if err := items.RewriteStatus(opts.ItemPath, "done", s.now()); err != nil {
			return fmt.Errorf("rewrite item status: %w", err)
		}
		dst, err := items.MoveToDone(opts.ItemPath, opts.DoneDir)
		if err != nil {
			return fmt.Errorf("move to done dir: %w", err)
		}
		movedFrom, movedTo = opts.ItemPath, dst
	}

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
		// Compensate: move the file back so the user can retry. Best
		// effort — if this fails the user has a file in done/ and an
		// active claim in the DB, but doctor's existing finders surface
		// that case and the user can squad force-release manually.
		if movedTo != "" && movedFrom != "" {
			_ = items.MoveBack(movedTo, filepath.Dir(movedFrom))
		}
		return dbErr
	}
	return nil
}
