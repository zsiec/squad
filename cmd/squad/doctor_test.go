package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/plugin/hooks"
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
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
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

func materializeAllHooksFor(t *testing.T, hookDir string) {
	t.Helper()
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entries, err := fs.ReadDir(hooks.FS, ".")
	if err != nil {
		t.Fatalf("read embed: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sh" {
			continue
		}
		body, err := fs.ReadFile(hooks.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(hookDir, e.Name()), body, 0o755); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}

func TestDoctor_ReportsHookDriftFromPluginRoot(t *testing.T) {
	run := setupDoctorRepo(t)

	pluginRoot := t.TempDir()
	hookDir := filepath.Join(pluginRoot, "hooks")
	materializeAllHooksFor(t, hookDir)
	if err := os.WriteFile(filepath.Join(hookDir, "session_start.sh"), []byte("# tampered\n"), 0o755); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", pluginRoot)

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "session_start.sh") || !strings.Contains(out, "modified") {
		t.Errorf("doctor stdout missing hook drift line: %s", out)
	}
}

func TestDoctor_StrictFailsOnHookDrift(t *testing.T) {
	run := setupDoctorRepo(t)

	pluginRoot := t.TempDir()
	hookDir := filepath.Join(pluginRoot, "hooks")
	materializeAllHooksFor(t, hookDir)
	if err := os.WriteFile(filepath.Join(hookDir, "session_start.sh"), []byte("# tampered\n"), 0o755); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", pluginRoot)

	out, err := run("doctor", "--strict")
	if err == nil {
		t.Fatalf("expected --strict to fail when hook drift present\nout=%s", out)
	}
}
