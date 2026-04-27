package intake

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/zsiec/squad/internal/items"
)

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
	"":         "FEAT",
	"feature":  "FEAT",
	"feat":     "FEAT",
	"bug":      "BUG",
	"task":     "TASK",
	"chore":    "CHORE",
	"tech-debt": "DEBT",
	"debt":     "DEBT",
	"bet":      "BET",
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
	if shape != ShapeItemOnly {
		return CommitResult{}, fmt.Errorf("intake commit: shape %q not yet supported (item_only only)", shape)
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

	return CommitResult{Shape: shape, ItemIDs: ids, Paths: paths}, nil
}

func loadSession(ctx context.Context, db *sql.DB, sessionID string) (Session, error) {
	var (
		s            Session
		refineItem   sql.NullString
		shape        sql.NullString
		createdAt    int64
		updatedAt    int64
		committedAt  sql.NullInt64
	)
	err := db.QueryRowContext(ctx, `
		SELECT id, repo_id, agent_id, mode, refine_item_id, idea_seed, status, shape,
		       created_at, updated_at, committed_at
		FROM intake_sessions WHERE id=?
	`, sessionID).Scan(
		&s.ID, &s.RepoID, &s.AgentID, &s.Mode,
		&refineItem, &s.IdeaSeed, &s.Status, &shape,
		&createdAt, &updatedAt, &committedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrIntakeNotFound
	}
	if err != nil {
		return Session{}, err
	}
	s.RefineItemID = refineItem.String
	s.Shape = shape.String
	s.CreatedAt = time.Unix(createdAt, 0).UTC()
	s.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	if committedAt.Valid {
		t := time.Unix(committedAt.Int64, 0).UTC()
		s.CommittedAt = &t
	}
	return s, nil
}
