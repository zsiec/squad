package repo

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func mustExec(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func TestDiscover_AutoRegistersOnFirstUse(t *testing.T) {
	dir := t.TempDir()
	mustExec(t, dir, "git", "init")
	mustExec(t, dir, "git", "remote", "add", "origin", "git@github.com:acme/example.git")

	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := NewRegistry(db)
	id, warn, err := DiscoverAndRegister(dir, r)
	if err != nil || warn != "" || id == "" {
		t.Fatalf("first: id=%q warn=%q err=%v", id, warn, err)
	}
	id2, warn2, err := DiscoverAndRegister(dir, r)
	if err != nil || id2 != id || warn2 != "" {
		t.Fatalf("idempotent failure: %q vs %q (warn=%q)", id, id2, warn2)
	}
}
