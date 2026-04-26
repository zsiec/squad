// Package store opens the operational SQLite database at ~/.squad/global.db,
// applies the embedded schema, and runs additive migrations. Higher-level
// packages (items, claims, chat, hygiene, attest, stats) own their own
// queries; this package just hands them a *sql.DB.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed schema.sql
var schemaSQL string

var additiveAlters = []string{
	`ALTER TABLE items ADD COLUMN epic_id TEXT`,
	`ALTER TABLE items ADD COLUMN parallel INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE items ADD COLUMN conflicts_with TEXT NOT NULL DEFAULT '[]'`,
	`ALTER TABLE attestations ADD COLUMN review_disagreements INTEGER NOT NULL DEFAULT 0`,
	`CREATE INDEX IF NOT EXISTS idx_items_epic ON items(repo_id, epic_id)`,
}

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_txlock=immediate", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	for _, stmt := range additiveAlters {
		if _, err := db.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				db.Close()
				return nil, fmt.Errorf("apply migration %q: %w", stmt, err)
			}
		}
	}
	return db, nil
}

func BeginImmediate(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin immediate: %w", err)
	}
	return tx, nil
}

const (
	txMaxRetries     = 6
	txInitialBackoff = 5 * time.Millisecond
	txMaxBackoff     = 250 * time.Millisecond
)

// errBusySentinel lets tests exercise the retry loop without colluding
// with real SQLite contention; isBusyError treats it as a busy signal.
var errBusySentinel = errors.New("store: simulated busy")

func WithTxRetry(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	backoff := txInitialBackoff
	var lastErr error
	for attempt := 0; attempt < txMaxRetries; attempt++ {
		err := func() error {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()
			if err := fn(tx); err != nil {
				return err
			}
			return tx.Commit()
		}()
		if err == nil {
			return nil
		}
		if !isBusyError(err) {
			return err
		}
		lastErr = err
		if attempt == txMaxRetries-1 {
			break
		}
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
		backoff *= 2
		if backoff > txMaxBackoff {
			backoff = txMaxBackoff
		}
	}
	return fmt.Errorf("withTxRetry: exhausted retries: %w", lastErr)
}

func isBusyError(err error) bool {
	if errors.Is(err, errBusySentinel) {
		return true
	}
	var sErr *sqlite.Error
	if errors.As(err, &sErr) {
		c := sErr.Code()
		return c == sqlite3.SQLITE_BUSY || c == sqlite3.SQLITE_LOCKED
	}
	return false
}
