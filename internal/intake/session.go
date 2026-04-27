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
)

const (
	StatusOpen      = "open"
	StatusCommitted = "committed"
	StatusCancelled = "cancelled"

	ModeNew    = "new"
	ModeRefine = "refine"
)

var (
	ErrIntakeNotFound      = errors.New("intake: session not found")
	ErrIntakeNotYours      = errors.New("intake: session owned by another agent")
	ErrIntakeAlreadyClosed = errors.New("intake: session already closed")
)

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
func Open(ctx context.Context, db *sql.DB, repoID, agentID, mode, ideaSeed string) (Session, bool, error) {
	if mode != ModeNew && mode != ModeRefine {
		return Session{}, false, fmt.Errorf("intake: invalid mode %q (want %q or %q)", mode, ModeNew, ModeRefine)
	}

	for attempt := 0; attempt < 2; attempt++ {
		if s, ok, err := findOpen(ctx, db, repoID, agentID); err != nil {
			return Session{}, false, err
		} else if ok {
			return s, true, nil
		}

		id, err := newSessionID(time.Now().UTC())
		if err != nil {
			return Session{}, false, err
		}
		now := time.Now().Unix()
		_, err = db.ExecContext(ctx, `
			INSERT INTO intake_sessions (id, repo_id, agent_id, mode, idea_seed, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'open', ?, ?)
		`, id, repoID, agentID, mode, ideaSeed, now, now)
		if err == nil {
			return Session{
				ID:        id,
				RepoID:    repoID,
				AgentID:   agentID,
				Mode:      mode,
				IdeaSeed:  ideaSeed,
				Status:    StatusOpen,
				CreatedAt: time.Unix(now, 0).UTC(),
				UpdatedAt: time.Unix(now, 0).UTC(),
			}, false, nil
		}
		if !isUniqueViolation(err) {
			return Session{}, false, err
		}
	}
	return Session{}, false, errors.New("intake: open contended after retry")
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
		s             Session
		refineItemID  sql.NullString
		shape         sql.NullString
		createdAt     int64
		updatedAt     int64
		committedAt   sql.NullInt64
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
