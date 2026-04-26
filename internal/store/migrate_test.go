package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func openEmptyDBNoMigrate(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrate_AppliesNewMigrationsInOrder(t *testing.T) {
	db := openEmptyDBNoMigrate(t)
	fsys := fstest.MapFS{
		"migrations/001_initial.sql": &fstest.MapFile{Data: []byte("CREATE TABLE foo (x INTEGER)")},
		"migrations/002_add_bar.sql": &fstest.MapFile{Data: []byte("ALTER TABLE foo ADD COLUMN bar TEXT")},
	}
	if err := Migrate(context.Background(), db, fsys); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var maxV int
	if err := db.QueryRow(`SELECT max(version) FROM migration_versions`).Scan(&maxV); err != nil {
		t.Fatalf("query: %v", err)
	}
	if maxV != 2 {
		t.Fatalf("want max version 2, got %d", maxV)
	}
}

func TestMigrate_SkipsAlreadyApplied(t *testing.T) {
	db := openEmptyDBNoMigrate(t)
	fsys := fstest.MapFS{
		"migrations/001_initial.sql": &fstest.MapFile{Data: []byte("CREATE TABLE foo (x INTEGER)")},
	}
	if err := Migrate(context.Background(), db, fsys); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(context.Background(), db, fsys); err != nil {
		t.Fatalf("second migrate (idempotent): %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT count(*) FROM migration_versions`).Scan(&rows); err != nil {
		t.Fatalf("query: %v", err)
	}
	if rows != 1 {
		t.Fatalf("want 1 row, got %d", rows)
	}
}

func TestMigrate_FailsCleanlyOnBadMigration(t *testing.T) {
	db := openEmptyDBNoMigrate(t)
	fsys := fstest.MapFS{
		"migrations/001_initial.sql": &fstest.MapFile{Data: []byte("CREATE TABLE foo (x INTEGER)")},
		"migrations/002_broken.sql":  &fstest.MapFile{Data: []byte("this is not valid sql")},
	}
	err := Migrate(context.Background(), db, fsys)
	if err == nil {
		t.Fatalf("want error from broken migration")
	}
	var maxV int
	_ = db.QueryRow(`SELECT COALESCE(max(version), 0) FROM migration_versions`).Scan(&maxV)
	if maxV != 1 {
		t.Fatalf("want migration 001 applied (version 1), got %d", maxV)
	}
}

func TestMigrate_SortsByVersionNotByName(t *testing.T) {
	db := openEmptyDBNoMigrate(t)
	fsys := fstest.MapFS{
		"migrations/010_z.sql": &fstest.MapFile{Data: []byte("ALTER TABLE foo ADD COLUMN c TEXT")},
		"migrations/002_b.sql": &fstest.MapFile{Data: []byte("ALTER TABLE foo ADD COLUMN b TEXT")},
		"migrations/001_a.sql": &fstest.MapFile{Data: []byte("CREATE TABLE foo (a TEXT)")},
	}
	if err := Migrate(context.Background(), db, fsys); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestMigrate_RejectsDuplicateVersionNumbers(t *testing.T) {
	db := openEmptyDBNoMigrate(t)
	fsys := fstest.MapFS{
		"migrations/001_a.sql": &fstest.MapFile{Data: []byte("CREATE TABLE foo (x INT)")},
		"migrations/01_b.sql":  &fstest.MapFile{Data: []byte("CREATE TABLE bar (x INT)")},
	}
	err := Migrate(context.Background(), db, fsys)
	if err == nil {
		t.Fatalf("want duplicate-version error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "duplicate version 1") {
		t.Fatalf("want error mentioning %q, got %q", "duplicate version 1", msg)
	}
}

func TestMigrate_BootstrapsLegacyDB(t *testing.T) {
	db := openEmptyDBNoMigrate(t)
	if err := Migrate(context.Background(), db, defaultMigrationsFS); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE migration_versions`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if err := Migrate(context.Background(), db, defaultMigrationsFS); err != nil {
		t.Fatalf("re-migrate must succeed: %v", err)
	}
	var maxV int
	if err := db.QueryRow(`SELECT max(version) FROM migration_versions`).Scan(&maxV); err != nil {
		t.Fatalf("query: %v", err)
	}
	if maxV < 3 {
		t.Fatalf("want at least version 3 seeded, got %d", maxV)
	}
}
