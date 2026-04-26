// Package store opens the operational SQLite database at ~/.squad/global.db,
// applies numbered migrations under internal/store/migrations/, and hands a
// *sql.DB to higher-level packages (items, claims, chat, hygiene, attest,
// stats) that own their own queries.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_txlock=immediate", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := Migrate(context.Background(), db, defaultMigrationsFS); err != nil {
		db.Close()
		return nil, err
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
