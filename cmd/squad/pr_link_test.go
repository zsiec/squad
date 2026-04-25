package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "items"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".squad", "done"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("project: test\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, ".squad", "items", "BUG-001-test.md"),
		[]byte("---\nid: BUG-001\nstatus: in-progress\n---\n\n# Test\n"),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}
	return dir
}

func runPRCmdInDir(t *testing.T, repoDir string, args ...string) (string, string, error) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	cmd := newPRLinkCmd()
	if args[0] == "pr-close" {
		cmd = newPRCloseCmd()
	}
	cmd.SetArgs(args[1:])
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmdErr := cmd.Execute()
	return out.String(), errBuf.String(), cmdErr
}

func TestPRLinkWritesPending(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())

	stdout, _, err := runPRCmdInDir(t, repo, "pr-link", "BUG-001")
	if err != nil {
		t.Fatalf("pr-link: %v", err)
	}
	if !strings.Contains(stdout, "<!-- squad-item: BUG-001 -->") {
		t.Fatalf("expected marker in stdout, got: %s", stdout)
	}

	pendingPath := filepath.Join(repo, ".squad", "pending-prs.json")
	b, err := os.ReadFile(pendingPath)
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	var entries []map[string]any
	if err := json.Unmarshal(b, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 1 || entries[0]["item_id"] != "BUG-001" {
		t.Fatalf("unexpected entries: %v", entries)
	}
}

func TestPRLinkUnknownItemErrors(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())

	_, _, err := runPRCmdInDir(t, repo, "pr-link", "GHOST-999")
	if err == nil {
		t.Fatalf("expected error for unknown item")
	}
	if !strings.Contains(err.Error(), "GHOST-999") {
		t.Fatalf("error should mention missing item; got: %v", err)
	}
}
