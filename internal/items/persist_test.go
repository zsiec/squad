package items

import (
	"context"
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
