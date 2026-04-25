package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStatus_PrintsCounts(t *testing.T) {
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
	write("BUG-001-ip.md", "---\nid: BUG-001\ntitle: ip\ntype: bug\npriority: P1\nstatus: in_progress\nestimate: 1h\n---\n")
	write("BUG-002-ready.md", "---\nid: BUG-002\ntitle: ready\ntype: bug\npriority: P2\nstatus: open\nestimate: 1h\n---\n")
	write("BUG-003-blocked.md", "---\nid: BUG-003\ntitle: blocked\ntype: bug\npriority: P1\nstatus: blocked\nestimate: 1h\n---\n")

	t.Setenv("SQUAD_HOME", filepath.Join(dir, "home"))
	t.Chdir(dir)
	var stdout bytes.Buffer
	code := runStatus(nil, &stdout)
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"in_progress: 1", "ready: 1", "blocked: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
