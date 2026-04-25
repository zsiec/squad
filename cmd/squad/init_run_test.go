package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInitDir(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
}

func TestRunInit_EmptyGitRepo_ScaffoldsAllFiles(t *testing.T) {
	repo := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	gitInitDir(t, repo)

	cmd := newInitCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--yes", "--dir", repo})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v\nstdout:\n%s", err, stdout.String())
	}

	for _, rel := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		".squad/config.yaml",
		".squad/STATUS.md",
		".squad/items/EXAMPLE-001-try-the-loop.md",
		".gitignore",
	} {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	out := stdout.String()
	for _, want := range []string{"Scaffolded", "squad next", "AGENTS.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("success message missing %q\nstdout:\n%s", want, out)
		}
	}
}

func TestRunInit_RerunIsIdempotent(t *testing.T) {
	repo := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	gitInitDir(t, repo)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repo})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	contents := snapshotDir(t, repo)

	second := newInitCmd()
	second.SetArgs([]string{"--yes", "--dir", repo})
	if err := second.Execute(); err != nil {
		t.Fatal(err)
	}
	contents2 := snapshotDir(t, repo)

	if len(contents) != len(contents2) {
		t.Fatalf("file set changed: %d -> %d", len(contents), len(contents2))
	}
	for path, body := range contents {
		if body != contents2[path] {
			t.Fatalf("file %s changed across runs", path)
		}
	}
}

func snapshotDir(t *testing.T, root string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.Contains(p, ".git/") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(b)
		return nil
	})
	return out
}
