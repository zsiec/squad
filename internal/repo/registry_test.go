package repo

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewRegistry(db)
}

func TestRepoID_StableForSameRemote(t *testing.T) {
	a := RepoID("git@github.com:acme/widgets.git", "/home/u/dev/widgets")
	b := RepoID("git@github.com:acme/widgets.git", "/home/u/dev/widgets")
	if a != b || len(a) != 16 {
		t.Fatalf("RepoID unstable or wrong len: %q vs %q", a, b)
	}
}

func TestRepoID_DiffersAcrossRemotes(t *testing.T) {
	a := RepoID("git@github.com:acme/widgets.git", "/x")
	b := RepoID("git@github.com:acme/sprockets.git", "/y")
	if a == b {
		t.Fatalf("collision: %q", a)
	}
}

func TestRegisterRegistry_FirstCallReturnsBase(t *testing.T) {
	r := testRegistry(t)
	id, warn, err := r.Register("git@github.com:acme/widgets.git", "/r/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if warn != "" {
		t.Fatalf("first register should not warn: %q", warn)
	}
	if len(id) != 16 {
		t.Fatalf("unexpected id %q", id)
	}
}

func TestRegisterRegistry_SecondCloneSameRemoteGetsSuffix(t *testing.T) {
	r := testRegistry(t)
	first, warn1, err := r.Register("git@github.com:acme/widgets.git", "/r/widgets")
	if err != nil || warn1 != "" {
		t.Fatalf("first register: %v warn=%q", err, warn1)
	}
	second, warn2, err := r.Register("git@github.com:acme/widgets.git", "/r/widgets-clone")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(second, "_2") || !strings.Contains(warn2, "second clone") || first == second {
		t.Fatalf("want _2 suffix + warn, got id=%q warn=%q", second, warn2)
	}
}

func TestRegisterRegistry_SamePath_Idempotent(t *testing.T) {
	r := testRegistry(t)
	first, _, _ := r.Register("git@github.com:acme/widgets.git", "/r/widgets")
	second, warn, err := r.Register("git@github.com:acme/widgets.git", "/r/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("idempotent register changed id: %q -> %q", first, second)
	}
	if warn != "" {
		t.Fatalf("idempotent register should not warn: %q", warn)
	}
}
