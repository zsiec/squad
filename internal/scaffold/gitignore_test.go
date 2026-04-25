package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignore_NoFile_Creates(t *testing.T) {
	root := t.TempDir()
	if err := EnsureGitignore(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".squad/db-snapshot/", ".squad/backups/"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestEnsureGitignore_AppendsMissingLines(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644)
	if err := EnsureGitignore(root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	s := string(got)
	if !strings.Contains(s, "node_modules/") {
		t.Fatal("clobbered existing")
	}
	if !strings.Contains(s, ".squad/db-snapshot/") {
		t.Fatalf("missing line")
	}
}

func TestEnsureGitignore_DoesNotDuplicate(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".squad/db-snapshot/\n.squad/backups/\n"), 0o644)
	if err := EnsureGitignore(root); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if strings.Count(string(got), ".squad/db-snapshot/") != 1 {
		t.Fatalf("duplicated line")
	}
}
