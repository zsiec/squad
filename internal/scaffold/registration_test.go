package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapAndRegister_CreatesDBAndInsertsRepo(t *testing.T) {
	stateDir := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("SQUAD_HOME", stateDir)

	got, err := BootstrapAndRegister(repoRoot, "https://github.com/me/octopus.git", "octopus")
	if err != nil {
		t.Fatalf("BootstrapAndRegister: %v", err)
	}
	if got.RepoID == "" {
		t.Fatal("empty repo id")
	}
	if _, err := os.Stat(filepath.Join(stateDir, "global.db")); err != nil {
		t.Fatalf("global.db missing: %v", err)
	}
}

func TestBootstrapAndRegister_Idempotent(t *testing.T) {
	stateDir := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("SQUAD_HOME", stateDir)

	first, err := BootstrapAndRegister(repoRoot, "git@github.com:me/p.git", "p")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BootstrapAndRegister(repoRoot, "git@github.com:me/p.git", "p")
	if err != nil {
		t.Fatal(err)
	}
	if first.RepoID != second.RepoID {
		t.Fatalf("repo id changed across runs: %q vs %q", first.RepoID, second.RepoID)
	}
}
