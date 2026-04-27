// Package commitlog records the commits an agent made during a claim,
// captured at squad-done time. The dashboard reads from here instead of
// shelling to git log on every drawer open.
package commitlog

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/zsiec/squad/internal/store"
)

// RecordSinceClaim enumerates every commit on HEAD with a commit
// timestamp at or after claimedAt and inserts each into the commits
// table. Returns how many new rows landed (existing rows are skipped via
// ON CONFLICT — re-running done is a no-op).
//
// Time-based; trusts the agent's working tree. Concurrent commits from
// peer agents on the same working tree branch in the same window will
// be attributed to whichever agent calls done first; multi-agent
// shared-tree workflows are out of scope.
func RecordSinceClaim(ctx context.Context, db *sql.DB, repoID, repoRoot, itemID, agentID string, claimedAt int64) (int, error) {
	if repoRoot == "" {
		return 0, nil
	}
	args := []string{
		"-C", repoRoot, "log",
		"--no-color",
		"--format=%H%x09%ct%x09%s",
		"--since=" + strconv.FormatInt(claimedAt, 10),
		"HEAD",
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("git log: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	type row struct {
		sha     string
		ts      int64
		subject string
	}
	var rows []row
	for _, line := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		ts, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		if ts < claimedAt {
			continue
		}
		rows = append(rows, row{sha: parts[0], ts: ts, subject: parts[2]})
	}
	if len(rows) == 0 {
		return 0, nil
	}

	inserted := 0
	err := store.WithTxRetry(ctx, db, func(tx *sql.Tx) error {
		inserted = 0
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO commits (repo_id, item_id, sha, subject, ts, agent_id)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (repo_id, sha) DO NOTHING
		`)
		if err != nil {
			return err
		}
		defer func() { _ = stmt.Close() }()
		for _, r := range rows {
			res, err := stmt.ExecContext(ctx, repoID, itemID, r.sha, r.subject, r.ts, agentID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				inserted++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return inserted, nil
}

// Commit is one row from the commits table — the squad-recorded metadata
// for a git commit attributed to an item's claim window.
type Commit struct {
	Sha     string
	Subject string
	TS      int64
	AgentID string
}

// ListForItem returns every recorded commit for an item, newest first.
// repoID == "" widens the query to all repos (workspace mode).
func ListForItem(ctx context.Context, db *sql.DB, repoID, itemID string) ([]Commit, error) {
	q := `SELECT sha, subject, ts, agent_id FROM commits WHERE item_id = ?`
	args := []any{itemID}
	if repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY ts DESC`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Commit
	for rows.Next() {
		var c Commit
		if err := rows.Scan(&c.Sha, &c.Subject, &c.TS, &c.AgentID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
