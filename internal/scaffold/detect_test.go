package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
}

func TestDetectRepo_FindsGitRootFromSubdirectory(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectRepo(sub)
	if err != nil {
		t.Fatalf("DetectRepo: %v", err)
	}
	// On macOS the temp dir resolves through /private/var; just check git root suffix.
	if !filepath.IsAbs(got.GitRoot) || filepath.Base(got.GitRoot) != filepath.Base(root) {
		t.Fatalf("git root mismatch: want suffix %q got %q", root, got.GitRoot)
	}
}

func TestDetectRepo_NoRemoteIsNotAnError(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	got, err := DetectRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Remote != "" {
		t.Fatalf("expected empty remote, got %q", got.Remote)
	}
}

func TestDetectRepo_PrimaryLanguageGo(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := DetectRepo(root)
	if got.PrimaryLanguage != "go" {
		t.Fatalf("want go, got %q", got.PrimaryLanguage)
	}
}

func TestDetectRepo_PrimaryLanguageNode(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0o644)
	got, _ := DetectRepo(root)
	if got.PrimaryLanguage != "node" {
		t.Fatalf("want node, got %q", got.PrimaryLanguage)
	}
}

func TestDetectRepo_PrimaryLanguageRust(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	_ = os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(""), 0o644)
	got, _ := DetectRepo(root)
	if got.PrimaryLanguage != "rust" {
		t.Fatalf("want rust, got %q", got.PrimaryLanguage)
	}
}

func TestDetectRepo_PrimaryLanguagePython(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	_ = os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte(""), 0o644)
	got, _ := DetectRepo(root)
	if got.PrimaryLanguage != "python" {
		t.Fatalf("want python, got %q", got.PrimaryLanguage)
	}
}

func TestDetectRepo_NotAGitRepoErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := DetectRepo(dir); err == nil {
		t.Fatal("expected error for non-git dir")
	}
}
