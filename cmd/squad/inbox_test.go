package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

const inboxItemReadyByA = `---
id: FEAT-201
title: Capture inbox triage flow end to end
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-A
captured_at: 1714150000
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

const inboxItemReadyByB = `---
id: FEAT-202
title: A second captured item that B authored for the inbox
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-B
captured_at: 1714150000
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

const inboxItemDoRFails = `---
id: FEAT-203
title: tiny
type: feature
priority: P1
area: <fill-in>
status: captured
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-A
captured_at: 1714150000
---

## Acceptance criteria
no checkboxes here
`

const inboxItemOpen = `---
id: FEAT-301
title: Already open should not show in inbox listing
type: feature
priority: P1
area: auth
status: open
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

func setupInboxRepo(t *testing.T, sessionID string) string {
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

func persistInboxFixture(t *testing.T, repoDir, fileName string) {
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

func TestRunInbox_EmptyPrintsFriendlyMessage(t *testing.T) {
	setupInboxRepo(t, "test-inbox-empty")

	var stdout, stderr bytes.Buffer
	code := runInbox(context.Background(), inboxOpts{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "inbox: empty") {
		t.Fatalf("stdout missing 'inbox: empty': %s", stdout.String())
	}
}

func TestRunInbox_ListsCapturedOnly(t *testing.T) {
	repoDir := setupInboxRepo(t, "test-inbox-listing")
	writeItemFile(t, repoDir, "FEAT-201-ready.md", inboxItemReadyByA)
	writeItemFile(t, repoDir, "FEAT-202-other.md", inboxItemReadyByB)
	writeItemFile(t, repoDir, "FEAT-301-open.md", inboxItemOpen)
	persistInboxFixture(t, repoDir, "FEAT-201-ready.md")
	persistInboxFixture(t, repoDir, "FEAT-202-other.md")
	persistInboxFixture(t, repoDir, "FEAT-301-open.md")

	var stdout, stderr bytes.Buffer
	code := runInbox(context.Background(), inboxOpts{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "FEAT-201") {
		t.Fatalf("stdout missing FEAT-201: %s", out)
	}
	if !strings.Contains(out, "FEAT-202") {
		t.Fatalf("stdout missing FEAT-202: %s", out)
	}
	if strings.Contains(out, "FEAT-301") {
		t.Fatalf("stdout should not include open item FEAT-301: %s", out)
	}
	for _, col := range []string{"ID", "AGE", "CAPTURED_BY", "DOR", "TITLE"} {
		if !strings.Contains(out, col) {
			t.Fatalf("stdout missing column %q: %s", col, out)
		}
	}
}

func TestRunInbox_MineFiltersByCurrentAgent(t *testing.T) {
	repoDir := setupInboxRepo(t, "test-inbox-mine")
	t.Setenv("SQUAD_AGENT", "agent-A")
	writeItemFile(t, repoDir, "FEAT-201-ready.md", inboxItemReadyByA)
	writeItemFile(t, repoDir, "FEAT-202-other.md", inboxItemReadyByB)
	persistInboxFixture(t, repoDir, "FEAT-201-ready.md")
	persistInboxFixture(t, repoDir, "FEAT-202-other.md")

	var stdout, stderr bytes.Buffer
	code := runInbox(context.Background(), inboxOpts{Mine: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "FEAT-201") {
		t.Fatalf("--mine should include agent-A's FEAT-201: %s", out)
	}
	if strings.Contains(out, "FEAT-202") {
		t.Fatalf("--mine should exclude agent-B's FEAT-202: %s", out)
	}
}

func TestRunInbox_ReadyOnlyFiltersFailingDoR(t *testing.T) {
	repoDir := setupInboxRepo(t, "test-inbox-ready")
	writeItemFile(t, repoDir, "FEAT-201-ready.md", inboxItemReadyByA)
	writeItemFile(t, repoDir, "FEAT-203-bad.md", inboxItemDoRFails)
	persistInboxFixture(t, repoDir, "FEAT-201-ready.md")
	persistInboxFixture(t, repoDir, "FEAT-203-bad.md")

	var stdout, stderr bytes.Buffer
	code := runInbox(context.Background(), inboxOpts{ReadyOnly: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "FEAT-201") {
		t.Fatalf("--ready-only should include passing FEAT-201: %s", out)
	}
	if strings.Contains(out, "FEAT-203") {
		t.Fatalf("--ready-only should exclude failing FEAT-203: %s", out)
	}
}

func TestRunInbox_RejectedTailsLog(t *testing.T) {
	repoDir := setupInboxRepo(t, "test-inbox-rejected")
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	logPath := filepath.Join(root, ".squad", "rejected.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := strings.Join([]string{
		`{"ts":1714150000,"id":"FEAT-501","title":"first","reason":"out of scope","by":"agent-A"}`,
		`{"ts":1714150500,"id":"FEAT-502","title":"second","reason":"duplicate","by":"agent-B"}`,
		"",
	}, "\n")
	if err := os.WriteFile(logPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runInbox(context.Background(), inboxOpts{Rejected: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"FEAT-501", "FEAT-502", "out of scope", "duplicate", "agent-A", "agent-B"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunInbox_RejectedMissingLogIsFriendly(t *testing.T) {
	setupInboxRepo(t, "test-inbox-no-log")

	var stdout, stderr bytes.Buffer
	code := runInbox(context.Background(), inboxOpts{Rejected: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "no rejections logged") {
		t.Fatalf("stdout missing 'no rejections logged': %s", stdout.String())
	}
}

func TestRunInbox_RejectedMutexCaughtAtParseTime(t *testing.T) {
	setupInboxRepo(t, "test-inbox-mutex")

	for _, conflict := range []string{"--mine", "--ready-only"} {
		t.Run(conflict, func(t *testing.T) {
			cmd := newInboxCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--rejected", conflict})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error pairing --rejected with %s; stdout=%s stderr=%s",
					conflict, stdout.String(), stderr.String())
			}
			if !strings.Contains(strings.ToLower(err.Error()), "mutually exclusive") {
				t.Fatalf("expected mutually-exclusive error, got: %v", err)
			}
		})
	}
}
