package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyze_WritesArtifact(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(p, body string) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(filepath.Join(dir, ".git", "HEAD"), "")
	mustWrite(filepath.Join(dir, ".squad", "config.yaml"), "")
	mustWrite(filepath.Join(dir, ".squad", "specs", "x.md"),
		"---\ntitle: X\nmotivation: y\nacceptance: [y]\n---\n")
	mustWrite(filepath.Join(dir, ".squad", "epics", "demo.md"),
		"---\nspec: x\nstatus: open\n---\n")
	mkItem := func(id, glob string) {
		mustWrite(filepath.Join(dir, ".squad", "items", id+".md"),
			"---\nid: "+id+"\ntitle: t\ntype: feature\npriority: P1\narea: core\n"+
				"status: open\nestimate: 1h\nrisk: low\ncreated: 2026-04-25\n"+
				"updated: 2026-04-25\nepic: demo\nparallel: true\n"+
				"conflicts_with:\n  - "+glob+"\n---\n\n## Problem\nx\n")
	}
	mkItem("FEAT-001", "internal/a.go")
	mkItem("FEAT-002", "internal/a.go")
	mkItem("FEAT-003", "internal/b.go")
	mkItem("FEAT-004", "internal/c.go")
	mkItem("FEAT-005", "internal/c.go")

	t.Chdir(dir)

	var out bytes.Buffer
	if code := runAnalyze([]string{"demo"}, &out); code != 0 {
		t.Fatalf("exit=%d, out=%s", code, out.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, ".squad", "epics", "demo-analysis.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{
		"# Analysis: demo", "Streams: 3", "Parallelism factor:",
		"FEAT-001", "FEAT-002", "FEAT-003", "FEAT-004", "FEAT-005",
	} {
		if !strings.Contains(string(body), frag) {
			t.Errorf("missing %q in:\n%s", frag, body)
		}
	}
}

func TestAnalyze_RejectsUnknownEpic(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var out bytes.Buffer
	if code := runAnalyze([]string{"does-not-exist"}, &out); code == 0 {
		t.Fatal("expected nonzero exit on unknown epic")
	}
}
