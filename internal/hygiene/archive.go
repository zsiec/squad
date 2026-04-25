package hygiene

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Archive moves messages older than beforeUnix into a per-month SQLite file
// in dir, then deletes them from the source. Returns the number of moved
// rows and the destination path.
func Archive(ctx context.Context, src *sql.DB, repoID, dir string, beforeUnix int64) (int, string, error) {
	month := time.Unix(beforeUnix, 0).UTC().Format("2006-01")
	archivePath := filepath.Join(dir, "squad-"+month+".db")
	adb, err := sql.Open("sqlite", archivePath)
	if err != nil {
		return 0, "", fmt.Errorf("open archive: %w", err)
	}
	defer adb.Close()
	if _, err := adb.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo_id TEXT NOT NULL,
			ts INTEGER NOT NULL, agent_id TEXT NOT NULL,
			thread TEXT NOT NULL, kind TEXT NOT NULL,
			body TEXT, mentions TEXT, priority TEXT
		)`); err != nil {
		return 0, "", err
	}
	rows, err := src.QueryContext(ctx, `
		SELECT ts, agent_id, thread, kind, COALESCE(body, ''),
		       COALESCE(mentions, '[]'), priority
		FROM messages WHERE repo_id = ? AND ts < ?`, repoID, beforeUnix)
	if err != nil {
		return 0, "", err
	}
	type row struct {
		TS                                         int64
		Agent, Thread, Kind, Body, Mentions, Prior string
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.TS, &r.Agent, &r.Thread, &r.Kind, &r.Body, &r.Mentions, &r.Prior); err != nil {
			rows.Close()
			return 0, "", err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if len(batch) == 0 {
		return 0, archivePath, nil
	}

	tx, err := adb.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	for _, r := range batch {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			repoID, r.TS, r.Agent, r.Thread, r.Kind, r.Body, r.Mentions, r.Prior); err != nil {
			_ = tx.Rollback()
			return 0, "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	if _, err := src.ExecContext(ctx, `DELETE FROM messages WHERE repo_id = ? AND ts < ?`, repoID, beforeUnix); err != nil {
		return 0, "", err
	}
	return len(batch), archivePath, nil
}
