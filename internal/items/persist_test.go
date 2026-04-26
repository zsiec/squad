package items

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/zsiec/squad/internal/store"
)

func TestPersist_InsertsNewRow(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	it := Item{
		ID: "FEAT-001", Title: "test", Type: "feat", Priority: "high",
		Path: "/repo/.squad/items/FEAT-001.md",
	}
	if err := Persist(ctx, db, "repo-1", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM items WHERE repo_id=? AND item_id=?`, "repo-1", "FEAT-001").Scan(&title); err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "test" {
		t.Fatalf("want title=test got %q", title)
	}
}

func TestPersist_UpsertsExistingRow(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	it := Item{ID: "FEAT-001", Title: "v1", Path: "/x.md"}
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("p1: %v", err)
	}
	it.Title = "v2"
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("p2: %v", err)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM items WHERE item_id='FEAT-001'`).Scan(&title); err != nil {
		t.Fatalf("query: %v", err)
	}
	if title != "v2" {
		t.Fatalf("want v2 got %q", title)
	}
}

func TestPersist_ArchivedTrueSetsStatusDoneAndArchivedFlag(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	it := Item{ID: "FEAT-002", Title: "shipped", Status: "in_progress", Path: "/y.md"}
	if err := Persist(ctx, db, "r", it, true); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var status string
	var archived int
	if err := db.QueryRow(`SELECT status, archived FROM items WHERE item_id='FEAT-002'`).Scan(&status, &archived); err != nil {
		t.Fatalf("query: %v", err)
	}
	if status != "done" {
		t.Fatalf("want status=done got %q", status)
	}
	if archived != 1 {
		t.Fatalf("want archived=1 got %d", archived)
	}
}

func TestPersist_RetriesOnContention(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	it := Item{ID: "FEAT-003", Title: "race", Path: "/z.md"}
	const n = 5
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Persist(ctx, db, "r", it, false)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent persist: %v", err)
		}
	}
}

func TestPersist_PreservesR3Fields(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	it := Item{
		ID: "FEAT-004", Title: "with epic", Status: "open", Path: "/p",
		Epic: "auth-rework", Parallel: true,
		ConflictsWith: []string{"a.go", "b.go"},
	}
	if err := Persist(ctx, db, "repo-1", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var (
		epic    string
		par     int
		confRaw string
	)
	if err := db.QueryRow(
		`SELECT epic_id, parallel, conflicts_with FROM items WHERE item_id=?`,
		"FEAT-004").Scan(&epic, &par, &confRaw); err != nil {
		t.Fatalf("query: %v", err)
	}
	if epic != "auth-rework" || par != 1 || confRaw != `["a.go","b.go"]` {
		t.Errorf("got epic=%q parallel=%d conflicts=%q", epic, par, confRaw)
	}
}

func TestPersist_RoundTripsIntakeProvenance(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	it := Item{
		ID: "FEAT-100", Title: "x", Status: "captured",
		Path:       "/repo/.squad/items/FEAT-100.md",
		CapturedBy: "agent-9f3a",
		CapturedAt: 1714150000,
		ParentSpec: "auth-rotation",
	}
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var capBy, parentSpec sql.NullString
	var capAt sql.NullInt64
	if err := db.QueryRow(
		`SELECT captured_by, captured_at, parent_spec FROM items
         WHERE repo_id=? AND item_id=?`, "r", "FEAT-100",
	).Scan(&capBy, &capAt, &parentSpec); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !capBy.Valid || capBy.String != "agent-9f3a" {
		t.Fatalf("captured_by = %+v want {valid=true value=agent-9f3a}", capBy)
	}
	if !capAt.Valid || capAt.Int64 != 1714150000 {
		t.Fatalf("captured_at = %+v want {valid=true value=1714150000}", capAt)
	}
	if !parentSpec.Valid || parentSpec.String != "auth-rotation" {
		t.Fatalf("parent_spec = %+v", parentSpec)
	}
}

func TestPersist_EmptyProvenanceStoresAsNull(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	it := Item{
		ID: "FEAT-101", Title: "no-provenance", Status: "open",
		Path: "/repo/.squad/items/FEAT-101.md",
	}
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var capBy sql.NullString
	var capAt sql.NullInt64
	_ = db.QueryRow(`SELECT captured_by, captured_at FROM items WHERE item_id=?`, "FEAT-101").Scan(&capBy, &capAt)
	if capBy.Valid {
		t.Fatalf("captured_by should be NULL for item without provenance, got %+v", capBy)
	}
	if capAt.Valid {
		t.Fatalf("captured_at should be NULL for item without provenance, got %+v", capAt)
	}
}

func TestPersist_AcceptanceProvenanceUpdatesOnUpsert(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	it := Item{
		ID: "FEAT-102", Title: "promotable", Status: "captured",
		Path:       "/repo/.squad/items/FEAT-102.md",
		CapturedBy: "agent-A", CapturedAt: 100,
	}
	_ = Persist(ctx, db, "r", it, false)
	it.Status = "open"
	it.AcceptedBy = "agent-B"
	it.AcceptedAt = 200
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("re-persist: %v", err)
	}
	var accBy sql.NullString
	var accAt sql.NullInt64
	_ = db.QueryRow(`SELECT accepted_by, accepted_at FROM items WHERE item_id=?`, "FEAT-102").Scan(&accBy, &accAt)
	if !accBy.Valid || accBy.String != "agent-B" {
		t.Fatalf("accepted_by=%+v", accBy)
	}
	if !accAt.Valid || accAt.Int64 != 200 {
		t.Fatalf("accepted_at=%+v", accAt)
	}
}
