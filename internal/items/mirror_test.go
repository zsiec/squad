package items_test

import (
	"context"
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
