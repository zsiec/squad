package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const attestTestItemBody = `---
id: FEAT-001
title: Attest test fixture
type: feature
priority: P1
area: core
status: open
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test]
---

## Acceptance criteria
- [ ] does the thing
`

func TestAttest_TestKind_HappyPath(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestTestItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "test", "--command", "printf 'ok\\n'"})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest: %v\nout=%s", err, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "FEAT-001") {
		t.Errorf("output missing FEAT-001: %s", body)
	}
	if !strings.Contains(body, "test") {
		t.Errorf("output missing kind=test: %s", body)
	}

	attDir := filepath.Join(repoDir, ".squad", "attestations")
	entries, err := os.ReadDir(attDir)
	if err != nil {
		t.Fatalf("read attestations dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 attestation file, got %d", len(entries))
	}
}

func TestAttest_BadKind(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestTestItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "fabricated", "--command", "true"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid kind, got nil")
	}
	if !strings.Contains(err.Error(), "invalid kind") {
		t.Fatalf("error should contain 'invalid kind', got: %v", err)
	}
}
