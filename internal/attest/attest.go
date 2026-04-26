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
	"time"
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
