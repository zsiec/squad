package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

const r4LifecycleItemBody = `---
id: FEAT-001
title: R4 lifecycle fixture
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

func TestR4_FullLifecycle(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-r4-lifecycle-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	run := func(args ...string) (string, error) {
		var c *cobra.Command
		if args[0] == "init" {
			c = newInitCmd()
			args = args[1:]
		} else {
			c = newRootCmd()
		}
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}

	if out, err := run("init", "--yes", "--dir", repoDir); err != nil {
		t.Fatalf("init: %v\nout=%s", err, out)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(r4LifecycleItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	if out, err := run("claim", "FEAT-001"); err != nil {
		t.Fatalf("claim: %v\nout=%s", err, out)
	}

	out, err := run("attest", "--item", "FEAT-001", "--kind", "test", "--command", "printf 'PASS\\nok\\n'")
	if err != nil {
		t.Fatalf("attest test: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "exit=0") {
		t.Errorf("attest test stdout missing exit=0: %s", out)
	}

	findingsPath := filepath.Join(repoDir, "findings.txt")
	if err := os.WriteFile(findingsPath,
		[]byte("status: clean\ndisagreements: 0\nresolution: accepted\n---\nlooks good\n"),
		0o644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	out, err = run("attest", "--item", "FEAT-001", "--kind", "review", "--reviewer-agent", "agent-r", "--findings-file", findingsPath)
	if err != nil {
		t.Fatalf("attest review: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "exit=0") {
		t.Errorf("attest review stdout missing exit=0: %s", out)
	}

	out, err = run("done", "FEAT-001", "--skip-verify")
	if err != nil {
		t.Fatalf("done: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "done FEAT-001") {
		t.Errorf("done stdout missing 'done FEAT-001': %s", out)
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
	recs, err := L.ListForItem(context.Background(), "FEAT-001")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 attestation records, got %d: %+v", len(recs), recs)
	}
	kinds := map[attest.Kind]bool{}
	for _, r := range recs {
		kinds[r.Kind] = true
	}
	if !kinds[attest.KindTest] {
		t.Errorf("missing kind=test in records: %+v", recs)
	}
	if !kinds[attest.KindReview] {
		t.Errorf("missing kind=review in records: %+v", recs)
	}

	out, _ = run("doctor")
	if strings.Contains(out, "evidence_missing") {
		t.Errorf("doctor flagged evidence_missing on a properly attested item: %s", out)
	}
}

func TestR4_DoctorFlagsForceClosedItemThatLatersLosesAttestation(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-r4-tamper-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	run := func(args ...string) (string, error) {
		var c *cobra.Command
		if args[0] == "init" {
			c = newInitCmd()
			args = args[1:]
		} else {
			c = newRootCmd()
		}
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}

	if out, err := run("init", "--yes", "--dir", repoDir); err != nil {
		t.Fatalf("init: %v\nout=%s", err, out)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(r4LifecycleItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	if out, err := run("claim", "FEAT-001"); err != nil {
		t.Fatalf("claim: %v\nout=%s", err, out)
	}

	out, err := run("done", "FEAT-001", "--skip-verify", "--force")
	if err != nil {
		t.Fatalf("done --force: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "done FEAT-001") {
		t.Errorf("done stdout missing 'done FEAT-001': %s", out)
	}

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	canonical, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	repoID, err := repo.IDFor(canonical)
	if err != nil {
		t.Fatalf("repo id: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM attestations WHERE repo_id = ? AND item_id = ?`, repoID, "FEAT-001"); err != nil {
		t.Fatalf("delete attestations: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	out, _ = run("doctor")
	if !strings.Contains(out, "evidence_missing") {
		t.Errorf("doctor stdout missing evidence_missing: %s", out)
	}
	if !strings.Contains(out, "FEAT-001") {
		t.Errorf("doctor stdout missing FEAT-001: %s", out)
	}
}
