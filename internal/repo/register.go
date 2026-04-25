package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func RegisterRepo(ctx context.Context, db *sql.DB, rootPath, remoteURL, name string) (string, error) {
	id := DeriveRepoID(remoteURL, rootPath)
	tx, err := store.BeginImmediate(ctx, db)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO repos (id, root_path, remote_url, name, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			root_path  = excluded.root_path,
			remote_url = excluded.remote_url,
			name       = excluded.name
	`, id, rootPath, remoteURL, name, time.Now().Unix())
	if err != nil {
		return "", fmt.Errorf("upsert repo: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}
