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
	write("BUG-001-ready.md", "---\nid: BUG-001\ntitle: ready\ntype: bug\npriority: P1\nstatus: open\nestimate: 1h\n---\n")
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
	for _, want := range []string{"claimed: 0", "ready: 2", "blocked: 1", "done: 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRunStatus_SubtractsClaimedFromReady(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dir, ".squad", "items", "FEAT-001-x.md"),
		[]byte("---\nid: FEAT-001\ntitle: x\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "items", "FEAT-002-y.md"),
		[]byte("---\nid: FEAT-002\ntitle: y\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	home := filepath.Join(dir, "home")
	t.Setenv("SQUAD_HOME", home)
	t.Setenv("SQUAD_AGENT", "agent-test")
	t.Setenv("SQUAD_SESSION_ID", "test-status")
	t.Chdir(dir)

	// Register, claim FEAT-001, verify status reports it as claimed not ready.
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"register", "--as", "agent-test", "--name", "Tester"})
	if err := root.Execute(); err != nil {
		t.Fatalf("register: %v", err)
	}

	root2 := newRootCmd()
	root2.SetOut(&bytes.Buffer{})
	root2.SetErr(&bytes.Buffer{})
	root2.SetArgs([]string{"claim", "FEAT-001", "--intent", "test"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	var out bytes.Buffer
	if code := runStatus(nil, &out); code != 0 {
		t.Fatalf("status exit=%d", code)
	}
	body := out.String()
	if !strings.Contains(body, "claimed: 1") {
		t.Errorf("expected claimed: 1, got:\n%s", body)
	}
	if !strings.Contains(body, "ready: 1") {
		t.Errorf("expected ready: 1 (FEAT-001 claimed → only FEAT-002 ready), got:\n%s", body)
	}
}
