package intake

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/zsiec/squad/internal/items"
)

const (
	StatusOpen      = "open"
	StatusCommitted = "committed"
	StatusCancelled = "cancelled"

	ModeNew    = "new"
	ModeRefine = "refine"
)

var (
	ErrIntakeNotFound           = errors.New("intake: session not found")
	ErrIntakeNotYours           = errors.New("intake: session owned by another agent")
	ErrIntakeAlreadyClosed      = errors.New("intake: session already closed")
	ErrIntakeItemNotRefinable   = errors.New("intake: only captured or needs-refinement items can be refined")
	ErrIntakeRefineItemMismatch = errors.New("intake: open call's refine_item_id does not match the resumed session")
)

// ItemSnapshot is a value-typed copy of an existing item, taken at the
// moment the refine session opened. The interview reads from the snapshot
// rather than the live row so concurrent edits to the file on disk don't
// surface mid-conversation.
type ItemSnapshot struct {
	ID         string
	Title      string
	Type       string
	Priority   string
	Area       string
	Status     string
	ParentSpec string
	ParentEpic string
	Body       string
}

// OpenParams bundles the inputs to Open. RefineItemID + SquadDir are
// only consulted when Mode == ModeRefine.
type OpenParams struct {
	RepoID       string
	AgentID      string
	Mode         string
	IdeaSeed     string
	RefineItemID string
	SquadDir     string
}

type Session struct {
	ID           string
	RepoID       string
	AgentID      string
	Mode         string
	RefineItemID string
	IdeaSeed     string
	Status       string
	Shape        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CommittedAt  *time.Time
}

// Open returns the existing open session for (repoID, agentID) with
// resumed=true, or a freshly created one with resumed=false. Concurrent
// opens by the same agent collide on the partial unique index from
// migration 009; the loser re-reads and returns the winner's row.
//
// When Mode == ModeRefine, RefineItemID + SquadDir are required and the
// item is loaded from disk into ItemSnapshot. The item must exist
// (items.ErrItemNotFound on miss) and be in captured or needs-refinement
// status (ErrIntakeItemNotRefinable otherwise). For ModeNew the snapshot
// return is the zero value.
//
// On resume, the call's RefineItemID must match the session's pinned
// refine_item_id; mismatches return ErrIntakeRefineItemMismatch. That
// sentinel also fires when the existing session's Mode disagrees with
// the call's Mode (since ModeNew sessions persist refine_item_id="").
func Open(ctx context.Context, db *sql.DB, p OpenParams) (Session, ItemSnapshot, bool, error) {
	if p.Mode != ModeNew && p.Mode != ModeRefine {
		return Session{}, ItemSnapshot{}, false, fmt.Errorf("intake: invalid mode %q (want %q or %q)", p.Mode, ModeNew, ModeRefine)
	}

	var snapshot ItemSnapshot
	if p.Mode == ModeRefine {
		var err error
		snapshot, err = loadItemSnapshot(p.SquadDir, p.RefineItemID)
		if err != nil {
			return Session{}, ItemSnapshot{}, false, err
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		if s, ok, err := findOpen(ctx, db, p.RepoID, p.AgentID); err != nil {
			return Session{}, ItemSnapshot{}, false, err
		} else if ok {
			if s.RefineItemID != p.RefineItemID {
				return Session{}, ItemSnapshot{}, false, ErrIntakeRefineItemMismatch
			}
			return s, snapshot, true, nil
		}

		id, err := newSessionID(time.Now().UTC())
		if err != nil {
			return Session{}, ItemSnapshot{}, false, err
		}
		now := time.Now().Unix()
		var refineCol any
		if p.Mode == ModeRefine {
			refineCol = p.RefineItemID
		}
		_, err = db.ExecContext(ctx, `
			INSERT INTO intake_sessions (id, repo_id, agent_id, mode, refine_item_id, idea_seed, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'open', ?, ?)
		`, id, p.RepoID, p.AgentID, p.Mode, refineCol, p.IdeaSeed, now, now)
		if err == nil {
			return Session{
				ID:           id,
				RepoID:       p.RepoID,
				AgentID:      p.AgentID,
				Mode:         p.Mode,
				RefineItemID: p.RefineItemID,
				IdeaSeed:     p.IdeaSeed,
				Status:       StatusOpen,
				CreatedAt:    time.Unix(now, 0).UTC(),
				UpdatedAt:    time.Unix(now, 0).UTC(),
			}, snapshot, false, nil
		}
		if !isUniqueViolation(err) {
			return Session{}, ItemSnapshot{}, false, err
		}
	}
	return Session{}, ItemSnapshot{}, false, errors.New("intake: open contended after retry")
}

func loadItemSnapshot(squadDir, itemID string) (ItemSnapshot, error) {
	path, _, err := items.FindByID(squadDir, itemID)
	if err != nil {
		return ItemSnapshot{}, err
	}
	it, err := items.Parse(path)
	if err != nil {
		return ItemSnapshot{}, err
	}
	if it.Status != "captured" && it.Status != "needs-refinement" {
		return ItemSnapshot{}, fmt.Errorf("%w: %s is %q", ErrIntakeItemNotRefinable, itemID, it.Status)
	}
	return ItemSnapshot{
		ID:         it.ID,
		Title:      it.Title,
		Type:       it.Type,
		Priority:   it.Priority,
		Area:       it.Area,
		Status:     it.Status,
		ParentSpec: it.ParentSpec,
		ParentEpic: it.ParentEpic,
		Body:       it.Body,
	}, nil
}

// Cancel marks a session cancelled. Errors:
//   - ErrIntakeNotFound  — id does not exist
//   - ErrIntakeNotYours  — the session is owned by a different agent
//   - ErrIntakeAlreadyClosed — already committed or cancelled
func Cancel(ctx context.Context, db *sql.DB, sessionID, agentID string) error {
	var owner, status string
	err := db.QueryRowContext(ctx,
		`SELECT agent_id, status FROM intake_sessions WHERE id=?`, sessionID,
	).Scan(&owner, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrIntakeNotFound
	}
	if err != nil {
		return err
	}
	if owner != agentID {
		return ErrIntakeNotYours
	}
	if status != StatusOpen {
		return ErrIntakeAlreadyClosed
	}
	_, err = db.ExecContext(ctx,
		`UPDATE intake_sessions SET status='cancelled', updated_at=? WHERE id=?`,
		time.Now().Unix(), sessionID,
	)
	return err
}

func findOpen(ctx context.Context, db *sql.DB, repoID, agentID string) (Session, bool, error) {
	var (
		s            Session
		refineItemID sql.NullString
		shape        sql.NullString
		createdAt    int64
		updatedAt    int64
		committedAt  sql.NullInt64
	)
	err := db.QueryRowContext(ctx, `
		SELECT id, repo_id, agent_id, mode, refine_item_id, idea_seed, status, shape,
		       created_at, updated_at, committed_at
		FROM intake_sessions
		WHERE repo_id=? AND agent_id=? AND status='open'
	`, repoID, agentID).Scan(
		&s.ID, &s.RepoID, &s.AgentID, &s.Mode,
		&refineItemID, &s.IdeaSeed, &s.Status, &shape,
		&createdAt, &updatedAt, &committedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	s.RefineItemID = refineItemID.String
	s.Shape = shape.String
	s.CreatedAt = time.Unix(createdAt, 0).UTC()
	s.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	if committedAt.Valid {
		t := time.Unix(committedAt.Int64, 0).UTC()
		s.CommittedAt = &t
	}
	return s, true, nil
}

func newSessionID(t time.Time) (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("intake id entropy: %w", err)
	}
	return fmt.Sprintf("intake-%s-%s", t.Format("20060102"), hex.EncodeToString(b[:])), nil
}

func isUniqueViolation(err error) bool {
	var sErr *sqlite.Error
	if errors.As(err, &sErr) {
		return sErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY ||
			sErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}

func loadSession(ctx context.Context, db *sql.DB, sessionID string) (Session, error) {
	var (
		s           Session
		refineItem  sql.NullString
		shape       sql.NullString
		createdAt   int64
		updatedAt   int64
		committedAt sql.NullInt64
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
