package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func TestRunNew_WritesFileAndPrintsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)
	var stdout bytes.Buffer
	code := runNew([]string{"bug", "Plug a leak"}, &stdout, items.Options{})
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.HasSuffix(out, ".md") {
		t.Fatalf("expected path output, got %q", out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output file missing: %v", err)
	}
}

func TestRunNew_PersistsItemRowImmediately(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)
	var stdout bytes.Buffer
	code := runNew([]string{"bug", "Persist me"}, &stdout, items.Options{})
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}
	created := strings.TrimSpace(stdout.String())
	parsed, err := items.Parse(created)
	if err != nil {
		t.Fatalf("parse created file: %v", err)
	}

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open default db: %v", err)
	}
	defer db.Close()
	canonical, err := repo.Discover(dir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	repoID, err := repo.IDFor(canonical)
	if err != nil {
		t.Fatalf("repo id: %v", err)
	}
	var title string
	if err := db.QueryRow(
		`SELECT title FROM items WHERE repo_id=? AND item_id=?`,
		repoID, parsed.ID,
	).Scan(&title); err != nil {
		t.Fatalf("items row missing after runNew: %v", err)
	}
	if title != "Persist me" {
		t.Errorf("title=%q want %q", title, "Persist me")
	}
}

func TestRunNew_PropagatesPersistError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// Point SQUAD_HOME at a path whose parent is a regular file. EnsureHome
	// MkdirAll fails because a path component is not a directory, which makes
	// store.OpenDefault return an error. runNew must propagate that error
	// rather than silently swallow it (re-introducing the items-table lag
	// the persist hook was meant to fix).
	parentFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(parentFile, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(parentFile, "home"))
	t.Chdir(dir)

	var stdout bytes.Buffer
	code := runNew([]string{"bug", "Should fail loudly"}, &stdout, items.Options{})
	if code == 0 {
		t.Fatalf("expected non-zero exit when persist fails; got 0 with stdout %q", stdout.String())
	}
	// File should still exist on disk — the failure is in persist, not in the
	// disk write. Re-running idempotently must remain possible.
	matches, err := filepath.Glob(filepath.Join(dir, ".squad", "items", "BUG-*.md"))
	if err != nil {
		t.Fatalf("glob items: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one BUG-*.md left on disk, got %d: %v", len(matches), matches)
	}
}

func TestRunNew_RejectsUnknownPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"),
		[]byte("id_prefixes: [STORY]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)
	var stdout bytes.Buffer
	code := runNew([]string{"bug", "x"}, &stdout, items.Options{})
	if code == 0 {
		t.Fatalf("expected non-zero exit; got 0 with stdout %q", stdout.String())
	}
}
