package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// brokenItemBody — frontmatter blockedBy a missing item to trip a hygiene
// finding deterministically.
const brokenItemBody = `---
id: FEAT-901
title: Broken-ref test item
type: feat
status: filed
priority: P3
created: 2026-04-26
updated: 2026-04-26
blocked-by: ["FEAT-9999-does-not-exist"]
---
## Problem
seed
`

func setupDoctorRepo(t *testing.T) func(args ...string) (string, error) {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-doctor-"+t.Name())
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
		filepath.Join(repoDir, ".squad", "items", "FEAT-901-broken.md"),
		[]byte(brokenItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}
	return run
}

func TestDoctor_ExitsZeroOnFindingsByDefault(t *testing.T) {
	run := setupDoctorRepo(t)

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("expected exit 0 even with findings; got err=%v\nout=%s", err, out)
	}
	if !strings.Contains(out, "finding(s)") {
		t.Errorf("doctor stdout missing 'finding(s)': %s", out)
	}
}

func TestDoctor_StrictReturnsErrorOnFindings(t *testing.T) {
	run := setupDoctorRepo(t)

	out, err := run("doctor", "--strict")
	if err == nil {
		t.Fatalf("expected --strict to return error when findings present\nout=%s", out)
	}
	if !strings.Contains(out, "finding(s)") {
		t.Errorf("doctor stdout missing 'finding(s)': %s", out)
	}
}
