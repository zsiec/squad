package intake

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zsiec/squad/internal/items"
)

const refineSupersededOutcome = "intake_refine_superseded"

// CommitResult is what Commit returns on success.
type CommitResult struct {
	Shape   string
	ItemIDs []string
	Paths   []string
}

// itemWriter is the per-item file allocator. Production wraps
// items.NewWithOptions; tests inject a controlled failure.
type itemWriter func(squadDir, prefix, title string, opts items.Options) (string, error)

// itemPrefixFor maps an ItemDraft.Kind to the squad ID prefix. Defaults to
// FEAT when Kind is empty so a green-field interview doesn't have to teach
// users about prefixes. Duplicated from cmd/squad/new.go:typeToPrefix
// because internal/ may not import cmd/.
var itemPrefixFor = map[string]string{
	"":          "FEAT",
	"feature":   "FEAT",
	"feat":      "FEAT",
	"bug":       "BUG",
	"task":      "TASK",
	"chore":     "CHORE",
	"tech-debt": "DEBT",
	"debt":      "DEBT",
	"bet":       "BET",
}

// Commit validates bundle, allocates item IDs, writes the item files,
// inserts rows under one DB tx, and marks the intake session committed.
// All-or-nothing: any failure rolls back the tx and deletes any files
// this call wrote. Today only the item_only shape is supported; the
// spec_epic_items shape returns an error.
func Commit(ctx context.Context, db *sql.DB, squadDir, sessionID, agentID string, bundle Bundle, ready bool) (CommitResult, error) {
	return commitImpl(ctx, db, squadDir, sessionID, agentID, bundle, ready, items.NewWithOptions)
}

func commitImpl(
	ctx context.Context,
	db *sql.DB,
	squadDir, sessionID, agentID string,
	bundle Bundle,
	ready bool,
	write itemWriter,
) (CommitResult, error) {
	sess, err := loadSession(ctx, db, sessionID)
	if err != nil {
		return CommitResult{}, err
	}
	if sess.AgentID != agentID {
		return CommitResult{}, ErrIntakeNotYours
	}
	if sess.Status != StatusOpen {
		return CommitResult{}, ErrIntakeAlreadyClosed
	}

	checklist, err := LoadChecklist(squadDir)
	if err != nil {
		return CommitResult{}, fmt.Errorf("intake commit: load checklist: %w", err)
	}
	shape, err := Validate(bundle, sess.Mode, checklist)
	if err != nil {
		return CommitResult{}, err
	}
	if shape == ShapeSpecEpicItems {
		return commitSpecEpicItems(ctx, db, squadDir, sessionID, agentID, bundle, ready, write)
	}
	if shape != ShapeItemOnly {
		return CommitResult{}, fmt.Errorf("intake commit: shape %q not supported", shape)
	}

	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		return CommitResult{}, fmt.Errorf("intake commit: marshal bundle: %w", err)
	}

	itemOpts := items.Options{Ready: ready, CapturedBy: agentID}

	var (
		paths []string
		ids   []string
	)
	cleanup := func() {
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}

	for _, it := range bundle.Items {
		prefix, ok := itemPrefixFor[it.Kind]
		if !ok {
			prefix = "FEAT"
		}
		path, werr := write(squadDir, prefix, it.Title, itemOpts)
		if werr != nil {
			cleanup()
			return CommitResult{}, fmt.Errorf("intake commit: write item file: %w", werr)
		}
		paths = append(paths, path)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		cleanup()
		return CommitResult{}, fmt.Errorf("intake commit: begin tx: %w", err)
	}
	rollback := func() {
		_ = tx.Rollback()
		cleanup()
	}

	now := time.Now().Unix()
	for _, p := range paths {
		parsed, perr := items.Parse(p)
		if perr != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: parse %s: %w", p, perr)
		}
		if perr := items.PersistOne(ctx, tx, sess.RepoID, parsed, false, now); perr != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: persist row %s: %w", parsed.ID, perr)
		}
		if _, perr := tx.ExecContext(ctx,
			`UPDATE items SET intake_session_id=? WHERE repo_id=? AND item_id=?`,
			sessionID, sess.RepoID, parsed.ID,
		); perr != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: link session on %s: %w", parsed.ID, perr)
		}
		ids = append(ids, parsed.ID)
	}

	var pendingMove *archiveMove
	if sess.Mode == ModeRefine {
		mv, err := supersedeOriginal(ctx, tx, squadDir, sess, now)
		if err != nil {
			rollback()
			return CommitResult{}, err
		}
		pendingMove = &mv
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE intake_sessions
		SET status='committed', shape=?, bundle_json=?, committed_at=?, updated_at=?
		WHERE id=?
	`, shape, string(bundleJSON), now, now, sessionID); err != nil {
		rollback()
		return CommitResult{}, fmt.Errorf("intake commit: mark session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		// A late tx.Commit() failure is rare but real (e.g., WAL checkpoint
		// failing on a full disk). The rows MAY have durably landed; we
		// still cleanup files and surface the error, on the bet that
		// orphan files are easier for a human to spot than orphan rows.
		cleanup()
		return CommitResult{}, fmt.Errorf("intake commit: tx commit: %w", err)
	}

	// File moves happen post-commit on purpose: the DB row is already
	// authoritative for "the original is superseded", so a rename failure
	// here leaves the row pointing at the archive path while the file is
	// still at the original path. squad doctor can detect the mismatch via
	// the path column; the alternative ordering (rename before commit)
	// trades that for a worse case where a tx.Commit failure leaves the
	// file moved with no row update to match it.
	if pendingMove != nil {
		if err := os.MkdirAll(filepath.Dir(pendingMove.to), 0o755); err != nil {
			return CommitResult{Shape: shape, ItemIDs: ids, Paths: paths},
				fmt.Errorf("intake commit (refine): db committed but mkdir %s failed: %w (run squad doctor)",
					filepath.Dir(pendingMove.to), err)
		}
		if err := os.Rename(pendingMove.from, pendingMove.to); err != nil {
			return CommitResult{Shape: shape, ItemIDs: ids, Paths: paths},
				fmt.Errorf("intake commit (refine): db committed but archive rename %s -> %s failed: %w (run squad doctor)",
					pendingMove.from, pendingMove.to, err)
		}
	}

	return CommitResult{Shape: shape, ItemIDs: ids, Paths: paths}, nil
}

type archiveMove struct{ from, to string }

// supersedeOriginal performs the in-tx refine-mode work: re-verify the
// original item is still captured (file is the source of truth, matching
// items.Promote's pattern), update its row to done+archived with the
// archive path, and record a claim_history line. The actual file move
// happens after tx.Commit succeeds; this function returns the planned
// move so the caller can execute it.
func supersedeOriginal(
	ctx context.Context,
	tx *sql.Tx,
	squadDir string,
	sess Session,
	now int64,
) (archiveMove, error) {
	origPath, _, ferr := items.FindByID(squadDir, sess.RefineItemID)
	if ferr != nil {
		return archiveMove{}, fmt.Errorf("intake commit (refine): %w", ferr)
	}
	origItem, perr := items.Parse(origPath)
	if perr != nil {
		return archiveMove{}, fmt.Errorf("intake commit (refine): parse %s: %w", origPath, perr)
	}
	if origItem.Status != "captured" && origItem.Status != "needs-refinement" {
		return archiveMove{}, fmt.Errorf("%w: %s is %q", ErrIntakeItemNotRefinable, sess.RefineItemID, origItem.Status)
	}

	archivePath := filepath.Join(squadDir, "items", ".archive", filepath.Base(origPath))

	if _, err := tx.ExecContext(ctx,
		`UPDATE items SET status='done', archived=1, path=? WHERE repo_id=? AND item_id=?`,
		archivePath, sess.RepoID, sess.RefineItemID,
	); err != nil {
		return archiveMove{}, fmt.Errorf("intake commit (refine): mark original superseded: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO claim_history (repo_id, item_id, agent_id, claimed_at, released_at, outcome)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sess.RepoID, sess.RefineItemID, sess.AgentID, now, now, refineSupersededOutcome,
	); err != nil {
		return archiveMove{}, fmt.Errorf("intake commit (refine): record claim_history: %w", err)
	}

	return archiveMove{from: origPath, to: archivePath}, nil
}
