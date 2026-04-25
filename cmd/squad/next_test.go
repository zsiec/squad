package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNext_PrintsHighestPriorityReady(t *testing.T) {
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
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, ".squad", "items", name),
			[]byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BUG-001-low.md", "---\nid: BUG-001\ntitle: low\ntype: bug\npriority: P2\nstatus: open\nestimate: 1h\n---\n")
	write("FEAT-002-high.md", "---\nid: FEAT-002\ntitle: high\ntype: feature\npriority: P0\nstatus: open\nestimate: 2h\n---\n")

	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)
	var stdout bytes.Buffer
	code := runNext(nil, &stdout, false, 0, false)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "FEAT-002") {
		t.Fatalf("expected FEAT-002 in output, got %q", stdout.String())
	}
}

func TestRunNext_EmptyQueueReturnsError(t *testing.T) {
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
	code := runNext(nil, &stdout, false, 0, false)
	if code == 0 {
		t.Fatal("expected non-zero exit on empty queue")
	}
}
