package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

const statsDoneItemTest = `---
id: FEAT-001
title: Stats fixture
type: feature
priority: P1
area: core
status: done
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test]
---

## Acceptance criteria
- [ ] does the thing
`

const statsDoneItemTestReview = `---
id: FEAT-002
title: Stats fixture
type: feature
priority: P1
area: core
status: done
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test, review]
---

## Acceptance criteria
- [ ] does the thing
`

func TestStats_VerificationRate_NoData(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-stats-no-data")
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

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats", "verification-rate"})
	if err := root.Execute(); err != nil {
		t.Fatalf("stats: %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "verification-rate") {
		t.Errorf("output missing 'verification-rate': %s", out.String())
	}
}

func TestStats_VerificationRate_OneOfTwo(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-stats-one-of-two")
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

	doneDir := filepath.Join(repoDir, ".squad", "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatalf("mkdir done: %v", err)
	}
	if err := os.WriteFile(filepath.Join(doneDir, "FEAT-001-test.md"), []byte(statsDoneItemTest), 0o644); err != nil {
		t.Fatalf("write FEAT-001: %v", err)
	}
	if err := os.WriteFile(filepath.Join(doneDir, "FEAT-002-test.md"), []byte(statsDoneItemTestReview), 0o644); err != nil {
		t.Fatalf("write FEAT-002: %v", err)
	}

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	canonical, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	repoID, err := repo.IDFor(canonical)
	if err != nil {
		t.Fatalf("repo id: %v", err)
	}
	L := attest.New(db, repoID, nil)
	ctx := context.Background()
	if _, err := L.Insert(ctx, attest.Record{
		ItemID:     "FEAT-001",
		Kind:       attest.KindTest,
		Command:    "go test ./...",
		ExitCode:   0,
		OutputHash: "h1",
		OutputPath: "/tmp/h1.txt",
		AgentID:    "agent-a",
	}); err != nil {
		t.Fatalf("insert FEAT-001: %v", err)
	}
	if _, err := L.Insert(ctx, attest.Record{
		ItemID:     "FEAT-002",
		Kind:       attest.KindTest,
		Command:    "go test ./...",
		ExitCode:   0,
		OutputHash: "h2",
		OutputPath: "/tmp/h2.txt",
		AgentID:    "agent-a",
	}); err != nil {
		t.Fatalf("insert FEAT-002: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"stats", "verification-rate"})
	if err := root.Execute(); err != nil {
		t.Fatalf("stats: %v\nout=%s", err, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "1/2") {
		t.Errorf("output missing '1/2': %s", body)
	}
	if !strings.Contains(body, "50") {
		t.Errorf("output missing '50': %s", body)
	}
}
