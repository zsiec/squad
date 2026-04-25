package claims

import (
	"context"
	"database/sql"
	"time"
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
