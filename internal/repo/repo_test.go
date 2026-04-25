package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if root != dir {
		t.Fatalf("got %q want %q", root, dir)
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
	if root != dir {
		t.Fatalf("got %q want %q", root, dir)
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
