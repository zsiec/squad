//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleEndToEnd(t *testing.T) {
	bin := buildBinary(t)
	repo := setupTempRepo(t)
	squadHome := t.TempDir()

	writeItem(t, repo, "FEAT-001", `---
id: FEAT-001
title: end-to-end
status: ready
created: 2026-04-24
updated: 2026-04-24
---

## Problem
testing
`)

	run(t, repo, squadHome, bin, "claim", "FEAT-001", "--intent", "first claim")
	run(t, repo, squadHome, bin, "release", "FEAT-001", "--outcome", "released")
	run(t, repo, squadHome, bin, "claim", "FEAT-001", "--intent", "second claim")
	run(t, repo, squadHome, bin, "done", "FEAT-001", "--summary", "shipped")

	if _, err := os.Stat(filepath.Join(repo, ".squad", "items", "FEAT-001-end-to-end.md")); !os.IsNotExist(err) {
		t.Fatal("item file still in items/ after done")
	}
	moved, err := os.ReadFile(filepath.Join(repo, ".squad", "done", "FEAT-001-end-to-end.md"))
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if !strings.Contains(string(moved), "status: done") {
		t.Fatalf("status not rewritten: %s", moved)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "squad")
	out, err := exec.Command("go", "build", "-o", bin, "./").CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func setupTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeItem(t *testing.T, repo, id, body string) {
	t.Helper()
	path := filepath.Join(repo, ".squad", "items", id+"-end-to-end.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, squadHome, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SQUAD_AGENT=agent-test", "SQUAD_HOME="+squadHome)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("squad %v: %v\n%s", args, err, out)
	}
}
