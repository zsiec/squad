package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const doneEvidenceItemTwoKinds = `---
id: FEAT-001
title: Done evidence test fixture
type: feature
priority: P1
area: core
status: open
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test, review]
---

## Acceptance criteria
- [ ] does the thing
`

const doneEvidenceItemOneKind = `---
id: FEAT-001
title: Done evidence test fixture
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

func TestDone_BlockedWhenEvidenceMissing(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-done-blocked")
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
		[]byte(doneEvidenceItemTwoKinds),
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
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"done", "FEAT-001", "--skip-verify"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error blocking done, got nil\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	combined := err.Error() + stdout.String() + stderr.String()
	for _, want := range []string{"evidence_required", "test", "review"} {
		if !strings.Contains(combined, want) {
			t.Errorf("combined output missing %q: %s", want, combined)
		}
	}
}

func TestDone_ProceedsWhenEvidenceSatisfied(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-done-satisfied")
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
		[]byte(doneEvidenceItemOneKind),
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

	attest := newRootCmd()
	var attOut bytes.Buffer
	attest.SetOut(&attOut)
	attest.SetErr(&attOut)
	attest.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "test", "--command", "printf 'ok\\n'"})
	if err := attest.Execute(); err != nil {
		t.Fatalf("attest: %v\nout=%s", err, attOut.String())
	}

	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"done", "FEAT-001", "--skip-verify"})
	if err := root.Execute(); err != nil {
		t.Fatalf("done: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "done FEAT-001") {
		t.Errorf("stdout missing 'done FEAT-001': %s", stdout.String())
	}
}
