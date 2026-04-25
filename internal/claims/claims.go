package claims

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type Store struct {
	db     *sql.DB
	now    func() time.Time
	repoID string
}

func New(db *sql.DB, repoID string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{db: db, repoID: repoID, now: now}
}

func (s *Store) nowUnix() int64 { return s.now().Unix() }

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Claim(ctx context.Context, itemID, agentID, intent string, touches []string, long bool) error {
	now := s.nowUnix()
	longVal := 0
	if long {
		longVal = 1
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, s.repoID, itemID, agentID, now, now, intent, longVal)
		if err != nil {
			if isUniqueViolation(err) {
				return ErrClaimTaken
			}
			return fmt.Errorf("insert claim: %w", err)
		}
		for _, p := range touches {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO touches (repo_id, agent_id, item_id, path, started_at)
				VALUES (?, ?, ?, ?, ?)
			`, s.repoID, agentID, itemID, p, now); err != nil {
				return fmt.Errorf("insert touch: %w", err)
			}
		}
		body := claimBody(itemID, intent)
		if err := postSystemMessage(ctx, tx, s.repoID, now, agentID, "global", "claim", body, nil, "normal"); err != nil {
			return err
		}
		return postSystemMessage(ctx, tx, s.repoID, now, agentID, itemID, "claim", body, nil, "normal")
	})
}

func claimBody(itemID, intent string) string {
	body := "claimed " + itemID
	if intent != "" {
		body += ": " + intent
	}
	return body
}

func isUniqueViolation(err error) bool {
	var sErr *sqlite.Error
	if errors.As(err, &sErr) {
		return sErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY ||
			sErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}
