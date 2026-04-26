// Package attest owns the squad evidence ledger: durable rows in the
// attestations table plus the captured stdout/stderr they reference on disk.
// Each row binds (item_id, kind, command) to a sha256 of the artifact, so
// any later edit to the file is detectable by re-hashing.
package attest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type Kind string

const (
	KindTest      Kind = "test"
	KindLint      Kind = "lint"
	KindTypecheck Kind = "typecheck"
	KindBuild     Kind = "build"
	KindReview    Kind = "review"
	KindManual    Kind = "manual"
)

func (k Kind) Valid() bool {
	switch k {
	case KindTest, KindLint, KindTypecheck, KindBuild, KindReview, KindManual:
		return true
	}
	return false
}

type Record struct {
	ID         int64
	ItemID     string
	Kind       Kind
	Command    string
	ExitCode   int
	OutputHash string
	OutputPath string
	CreatedAt  int64
	AgentID    string
	RepoID     string
}

type Ledger struct {
	db     *sql.DB
	repoID string
	now    func() time.Time
}

func New(db *sql.DB, repoID string, now func() time.Time) *Ledger {
	if now == nil {
		now = time.Now
	}
	return &Ledger{db: db, repoID: repoID, now: now}
}

func (l *Ledger) nowUnix() int64 { return l.now().Unix() }

func (l *Ledger) Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (l *Ledger) Insert(ctx context.Context, r Record) (int64, error) {
	if !r.Kind.Valid() {
		return 0, fmt.Errorf("invalid kind %q (want test|lint|typecheck|build|review|manual)", r.Kind)
	}
	created := r.CreatedAt
	if created == 0 {
		created = l.nowUnix()
	}
	res, err := l.db.ExecContext(ctx, `
		INSERT INTO attestations (item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, repo_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		r.ItemID, string(r.Kind), r.Command, r.ExitCode,
		r.OutputHash, r.OutputPath, created, r.AgentID, l.repoID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			var existing int64
			if qerr := l.db.QueryRowContext(ctx,
				`SELECT id FROM attestations WHERE item_id = ? AND output_hash = ?`,
				r.ItemID, r.OutputHash).Scan(&existing); qerr == nil {
				return existing, nil
			}
		}
		return 0, fmt.Errorf("insert attestation: %w", err)
	}
	return res.LastInsertId()
}

func isUniqueViolation(err error) bool {
	var sErr *sqlite.Error
	if errors.As(err, &sErr) {
		return sErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY ||
			sErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}

func (l *Ledger) ListForItem(ctx context.Context, itemID string) ([]Record, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, repo_id
		FROM attestations
		WHERE repo_id = ? AND item_id = ?
		ORDER BY created_at ASC, id ASC
	`, l.repoID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var k string
		if err := rows.Scan(&r.ID, &r.ItemID, &k, &r.Command, &r.ExitCode, &r.OutputHash, &r.OutputPath, &r.CreatedAt, &r.AgentID, &r.RepoID); err != nil {
			return nil, err
		}
		r.Kind = Kind(k)
		out = append(out, r)
	}
	return out, rows.Err()
}
