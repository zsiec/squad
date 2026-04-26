package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
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
