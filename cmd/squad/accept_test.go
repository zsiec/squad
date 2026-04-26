package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

const acceptItemReady = `---
id: FEAT-001
title: Wire up the new accept verb plumbing for inbox
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] does the thing
`

const acceptItemDoRFails = `---
id: FEAT-002
title: tiny
type: feature
priority: P1
area: <fill-in>
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
no checkboxes here
`

func setupAcceptRepo(t *testing.T, sessionID string) string {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", sessionID)
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
	return repoDir
}

func persistAcceptFixture(t *testing.T, repoDir, fileName string) {
	t.Helper()
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("repo.Discover: %v", err)
	}
	path := filepath.Join(root, ".squad", "items", fileName)
	parsed, err := items.Parse(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	parsed.Path = path
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo.IDFor: %v", err)
	}
	if err := items.Persist(context.Background(), db, repoID, parsed, false); err != nil {
		t.Fatalf("persist %s: %v", fileName, err)
	}
}

func acceptItemStatus(t *testing.T, itemID string) string {
	t.Helper()
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	var status string
	if err := db.QueryRow(`SELECT status FROM items WHERE item_id=?`, itemID).Scan(&status); err != nil {
		t.Fatalf("query status for %s: %v", itemID, err)
	}
	return status
}

func TestRunAccept_PromotesPassingItem(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-accept-pass")
	writeItemFile(t, repoDir, "FEAT-001-ready.md", acceptItemReady)
	persistAcceptFixture(t, repoDir, "FEAT-001-ready.md")

	var stdout, stderr bytes.Buffer
	code := runAccept(context.Background(), []string{"FEAT-001"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "accepted FEAT-001") {
		t.Fatalf("stdout missing 'accepted FEAT-001': %s", stdout.String())
	}
	if got := acceptItemStatus(t, "FEAT-001"); got != "open" {
		t.Fatalf("status=%q want open", got)
	}
}

func TestRunAccept_PartialSuccess(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-accept-partial")
	writeItemFile(t, repoDir, "FEAT-001-ready.md", acceptItemReady)
	writeItemFile(t, repoDir, "FEAT-002-bad.md", acceptItemDoRFails)
	persistAcceptFixture(t, repoDir, "FEAT-001-ready.md")
	persistAcceptFixture(t, repoDir, "FEAT-002-bad.md")

	var stdout, stderr bytes.Buffer
	code := runAccept(context.Background(), []string{"FEAT-001", "FEAT-002"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (partial)\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "accepted FEAT-001") {
		t.Fatalf("stdout missing 'accepted FEAT-001': %s", stdout.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "FEAT-002") {
		t.Fatalf("stderr missing FEAT-002: %s", errOut)
	}
	if !strings.Contains(errOut, "area") {
		t.Fatalf("stderr missing 'area' DoR violation: %s", errOut)
	}
	if got := acceptItemStatus(t, "FEAT-001"); got != "open" {
		t.Fatalf("FEAT-001 status=%q want open", got)
	}
	if got := acceptItemStatus(t, "FEAT-002"); got != "captured" {
		t.Fatalf("FEAT-002 status=%q want captured (still)", got)
	}
}

func TestRunAccept_UnknownIDErrors(t *testing.T) {
	setupAcceptRepo(t, "test-accept-unknown")

	var stdout, stderr bytes.Buffer
	code := runAccept(context.Background(), []string{"FEAT-999"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit=0 want nonzero\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "FEAT-999") {
		t.Fatalf("stderr missing FEAT-999: %s", errOut)
	}
	if !strings.Contains(errOut, "not found") {
		t.Fatalf("stderr missing 'not found': %s", errOut)
	}
}

func TestRunAccept_NoIDsRejectedByCobra(t *testing.T) {
	setupAcceptRepo(t, "test-accept-no-args")

	cmd := newAcceptCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected cobra Args validator to reject empty arg list, got nil err\nstdout=%s\nstderr=%s",
			stdout.String(), stderr.String())
	}
}
