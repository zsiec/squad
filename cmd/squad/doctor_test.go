package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
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

// setupDoctorRepoBare initialises a repo + isolated SQUAD_HOME and returns
// (repoDir, run). It does NOT seed the FEAT-901 broken item — callers that
// rely on a pre-existing finding should use setupDoctorRepo instead.
func setupDoctorRepoBare(t *testing.T) (string, func(args ...string) (string, error)) {
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
	return repoDir, run
}

func setupDoctorRepo(t *testing.T) func(args ...string) (string, error) {
	t.Helper()
	repoDir, run := setupDoctorRepoBare(t)
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

// seedCapturedItem inserts a captured item directly into the global db for
// the test repo. Bypasses item-file authoring on purpose: the doctor checks
// query items by status/captured_at and don't need the on-disk file.
func seedCapturedItem(t *testing.T, repoDir, itemID string, capturedAt int64) {
	t.Helper()
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("repo.Discover: %v", err)
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo.IDFor: %v", err)
	}
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		INSERT INTO items (repo_id, item_id, title, type, priority, status, path, updated_at, captured_at)
		VALUES (?, ?, ?, 'feat', 'P3', 'captured', ?, ?, ?)
	`, repoID, itemID, itemID+" title",
		filepath.Join(".squad", "items", itemID+".md"),
		time.Now().Unix(), capturedAt); err != nil {
		t.Fatalf("seed item %s: %v", itemID, err)
	}
}

func TestDoctor_FlagsStaleCapture(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	seedCapturedItem(t, repoDir, "FEAT-OLD", old)

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "stale_capture") || !strings.Contains(out, "FEAT-OLD") {
		t.Fatalf("expected stale_capture finding for FEAT-OLD; got:\n%s", out)
	}
}

func TestDoctor_StrictFailsOnStaleCapture(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	seedCapturedItem(t, repoDir, "FEAT-OLD", old)

	out, err := run("doctor", "--strict")
	if err == nil {
		t.Fatalf("expected --strict to fail on stale capture\nout=%s", out)
	}
}

func TestDoctor_FlagsInboxOverflow(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	now := time.Now().Unix()
	for i := 0; i < 51; i++ {
		seedCapturedItem(t, repoDir, fmt.Sprintf("FEAT-%03d", i), now)
	}

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "inbox_overflow") {
		t.Fatalf("expected inbox_overflow finding; got:\n%s", out)
	}
}

func TestDoctor_FlagsRejectedLogOverflow(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	var b strings.Builder
	for i := 0; i < 501; i++ {
		fmt.Fprintf(&b, "rejection %d\n", i)
	}
	logPath := filepath.Join(repoDir, ".squad", "rejected.log")
	if err := os.WriteFile(logPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write rejected.log: %v", err)
	}

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "rejected_log_overflow") {
		t.Fatalf("expected rejected_log_overflow finding; got:\n%s", out)
	}
}
