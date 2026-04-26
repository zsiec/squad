package items

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
	_ "modernc.org/sqlite"
)

func seedPromotableItem(t *testing.T, ctx context.Context, db *sql.DB, squadDir, repoID, id string) Item {
	t.Helper()
	p := filepath.Join(squadDir, "items", id+"-thing-with-many-words.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
id: ` + id + `
title: investigate the flaky auth test we have
type: feat
status: captured
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-A
captured_at: 100
---

## Acceptance criteria
- [ ] does the thing
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, err := Parse(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := Persist(ctx, db, repoID, it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	return it
}

func TestPromote_HappyPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	seeded := seedPromotableItem(t, ctx, db, squadDir, "r", "FEAT-001")
	if err := Promote(ctx, db, "r", seeded.ID, "agent-B"); err != nil {
		t.Fatalf("promote: %v", err)
	}
	after, err := Parse(seeded.Path)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if after.Status != "open" {
		t.Fatalf("status=%q want open", after.Status)
	}
	if after.AcceptedBy != "agent-B" {
		t.Fatalf("AcceptedBy=%q want agent-B", after.AcceptedBy)
	}
	if after.AcceptedAt == 0 {
		t.Fatalf("AcceptedAt unset")
	}
	var dbStatus, dbAccBy string
	var dbAccAt int64
	if err := db.QueryRow(
		`SELECT status, COALESCE(accepted_by, ''), COALESCE(accepted_at, 0) FROM items WHERE item_id=?`,
		seeded.ID,
	).Scan(&dbStatus, &dbAccBy, &dbAccAt); err != nil {
		t.Fatalf("db scan: %v", err)
	}
	if dbStatus != "open" || dbAccBy != "agent-B" || dbAccAt == 0 {
		t.Fatalf("db row not updated: status=%q acceptedBy=%q acceptedAt=%d", dbStatus, dbAccBy, dbAccAt)
	}
}

func TestPromote_DoRViolationReturnsTypedError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	p := filepath.Join(squadDir, "items", "FEAT-002-thing.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
id: FEAT-002
title: investigate the flaky auth test we have
type: feat
status: captured
priority: P2
area: <fill-in>
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] x
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, _ := Parse(p)
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	err := Promote(ctx, db, "r", "FEAT-002", "agent-B")
	if err == nil {
		t.Fatalf("want DoR error, got nil")
	}
	var dorErr *DoRError
	if !errors.As(err, &dorErr) {
		t.Fatalf("err not *DoRError: %v", err)
	}
	if len(dorErr.Violations) == 0 {
		t.Fatalf("DoRError has no violations")
	}
	after, _ := Parse(p)
	if after.Status != "captured" {
		t.Fatalf("DoR-failing promote should not have rewritten file; status=%q", after.Status)
	}
}

func TestPromote_AlreadyOpenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	p := filepath.Join(squadDir, "items", "FEAT-003-thing.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
id: FEAT-003
title: investigate the flaky auth test we have
type: feat
status: open
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
accepted_by: agent-X
accepted_at: 12345
---

## Acceptance criteria
- [ ] x
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, _ := Parse(p)
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := Promote(ctx, db, "r", "FEAT-003", "agent-Y"); err != nil {
		t.Fatalf("idempotent promote should not error: %v", err)
	}
	after, _ := Parse(p)
	if after.AcceptedBy != "agent-X" {
		t.Fatalf("idempotent promote should NOT overwrite existing acceptance; AcceptedBy=%q", after.AcceptedBy)
	}
	if after.AcceptedAt != 12345 {
		t.Fatalf("idempotent promote should NOT overwrite acceptedAt; got %d", after.AcceptedAt)
	}
}

func TestPromote_MissingItemErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	err := Promote(ctx, db, "r", "FEAT-999", "agent-B")
	if err == nil {
		t.Fatalf("want error for missing item")
	}
}

func TestPromote_BlockedStatusErrors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	p := filepath.Join(squadDir, "items", "FEAT-004-thing.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
id: FEAT-004
title: investigate the flaky auth test we have
type: feat
status: blocked
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---
body
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, _ := Parse(p)
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := Promote(ctx, db, "r", "FEAT-004", "agent-B"); err == nil {
		t.Fatalf("want error promoting a blocked item")
	}
}
