package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
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
