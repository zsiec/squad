package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpen_AppliesSchemaIdempotently(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "global.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"repos", "agents", "claims", "claim_history", "messages", "touches", "reads", "progress"} {
		var name string
		if err := db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	db2.Close()
}

func TestOpen_RepoIdColumnPresent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"agents", "claims", "claim_history", "messages", "touches"} {
		rows, err := db.QueryContext(context.Background(),
			"SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", table, err)
		}
		found := false
		for rows.Next() {
			var col string
			if err := rows.Scan(&col); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if col == "repo_id" {
				found = true
			}
		}
		rows.Close()
		if !found {
			t.Fatalf("table %s missing repo_id column", table)
		}
	}
}
