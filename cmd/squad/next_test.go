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

func TestNext_OmitsItemsWithUnmetDependsOn(t *testing.T) {
	dir := t.TempDir()
	w := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hdr := func(id, prio, deps string) string {
		return "---\nid: " + id + "\ntitle: t\ntype: feature\npriority: " + prio +
			"\narea: core\nstatus: open\nestimate: 1h\nrisk: low\n" +
			"created: 2026-04-25\nupdated: 2026-04-25\n" + deps + "---\n\n## Problem\nx\n"
	}
	w(filepath.Join(dir, ".git", "HEAD"), "")
	w(filepath.Join(dir, ".squad", "config.yaml"), "")
	w(filepath.Join(dir, ".squad", "items", "FEAT-500.md"), hdr("FEAT-500", "P1", ""))
	w(filepath.Join(dir, ".squad", "items", "FEAT-501.md"),
		hdr("FEAT-501", "P0", "depends_on:\n  - FEAT-500\n"))

	t.Setenv("SQUAD_HOME", filepath.Join(dir, "squad-home"))
	t.Chdir(dir)

	var out bytes.Buffer
	if code := runNext(nil, &out, false, 0, false); code != 0 {
		t.Fatalf("exit=%d, out=%s", code, out.String())
	}
	body := out.String()
	if strings.Contains(body, "FEAT-501") {
		t.Errorf("FEAT-501 should be hidden\n%s", body)
	}
	if !strings.Contains(body, "FEAT-500") {
		t.Errorf("FEAT-500 should appear\n%s", body)
	}
}
