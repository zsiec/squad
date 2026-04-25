package items_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/store"
)

func TestMirror_UpsertsOneRowPerItem(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	w, err := items.Walk("testdata/ready")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := items.Mirror(ctx, db, "repo-abc", w); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM items WHERE repo_id = ?`, "repo-abc").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(w.Active)+len(w.Done) {
		t.Fatalf("rows=%d want %d", n, len(w.Active)+len(w.Done))
	}

	if err := items.Mirror(ctx, db, "repo-abc", w); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM items WHERE repo_id = ?`, "repo-abc").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(w.Active)+len(w.Done) {
		t.Fatalf("after second mirror rows=%d want %d", n, len(w.Active)+len(w.Done))
	}
}

func TestMirror_PersistsR3Fields(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	w := items.WalkResult{Active: []items.Item{{
		ID: "FEAT-200", Title: "x", Status: "open", Path: "/p",
		Epic: "auth-rework", Parallel: true,
		ConflictsWith: []string{"a.go", "b.go"},
	}}}
	if err := items.Mirror(context.Background(), db, "repo-1", w); err != nil {
		t.Fatal(err)
	}
	var (
		epic    sql.NullString
		par     int
		confRaw string
	)
	if err := db.QueryRow(
		`SELECT epic_id, parallel, conflicts_with FROM items WHERE item_id=?`,
		"FEAT-200").Scan(&epic, &par, &confRaw); err != nil {
		t.Fatal(err)
	}
	if epic.String != "auth-rework" || par != 1 || confRaw != `["a.go","b.go"]` {
		t.Errorf("got epic=%q parallel=%d conflicts=%q", epic.String, par, confRaw)
	}
}
