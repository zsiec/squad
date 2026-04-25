package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func TestDeriveRepoID_StableForSameRemote(t *testing.T) {
	a := DeriveRepoID("git@github.com:foo/bar.git", "/tmp/a")
	b := DeriveRepoID("git@github.com:foo/bar.git", "/tmp/b")
	if a != b {
		t.Fatalf("same remote should yield same id: %q vs %q", a, b)
	}
	if len(a) != 16 {
		t.Fatalf("repo_id should be 16 hex chars, got %q (len=%d)", a, len(a))
	}
}

func TestDeriveRepoID_DiffersForDifferentRemotes(t *testing.T) {
	a := DeriveRepoID("git@github.com:foo/bar.git", "/tmp/x")
	b := DeriveRepoID("git@github.com:foo/baz.git", "/tmp/x")
	if a == b {
		t.Fatal("different remotes should yield different ids")
	}
}

func TestDeriveRepoID_PathDiscriminatorWhenRemoteEmpty(t *testing.T) {
	a := DeriveRepoID("", "/tmp/x")
	b := DeriveRepoID("", "/tmp/y")
	if a == b {
		t.Fatal("empty remote with different paths should differ")
	}
}

func TestDiscover_FindsConfigInCWD(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical, _ := filepath.EvalSymlinks(dir)
	if root != wantCanonical {
		t.Fatalf("got %q want %q", root, wantCanonical)
	}
}

func TestDiscover_WalksUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := Discover(deep)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical, _ := filepath.EvalSymlinks(dir)
	if root != wantCanonical {
		t.Fatalf("got %q want %q", root, wantCanonical)
	}
}

func TestDiscover_CanonicalizesSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "via-symlink")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("symlink not supported on this platform")
	}

	rootViaLink, err := Discover(link)
	if err != nil {
		t.Fatal(err)
	}
	rootViaReal, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rootViaLink != rootViaReal {
		t.Fatalf("symlinked path resolves differently than canonical:\n  via link: %q\n  via real: %q", rootViaLink, rootViaReal)
	}
}

func TestDiscover_ErrorWithHelpfulMessage(t *testing.T) {
	dir := t.TempDir()
	_, err := Discover(dir)
	if err == nil {
		t.Fatal("expected error when no .squad/config.yaml found")
	}
	if !strings.Contains(err.Error(), "squad init") {
		t.Fatalf("error should mention `squad init`: %v", err)
	}
}

func TestRegisterRepo_InsertsThenIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	if err := store.EnsureHome(); err != nil {
		t.Fatal(err)
	}
	dbPath, _ := store.DBPath()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := RegisterRepo(context.Background(), db, "/tmp/myproj", "git@github.com:foo/bar.git", "myproj")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty repo id")
	}
	id2, err := RegisterRepo(context.Background(), db, "/tmp/myproj", "git@github.com:foo/bar.git", "myproj")
	if err != nil {
		t.Fatal(err)
	}
	if id != id2 {
		t.Fatalf("idempotent register changed id: %q -> %q", id, id2)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM repos WHERE id=?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d rows, want 1", count)
	}
}
