package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/repo"
)

const refineItemCaptured = `---
id: FEAT-401
title: Refine me — make the criteria sharper please
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-A
captured_at: 1714150000
---

## Problem

Auth flow is racy.

## Acceptance criteria
- [ ] the rule does the thing as specified
`

const refineItemSecondCaptured = `---
id: FEAT-402
title: A second captured item left untouched by the test
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-B
captured_at: 1714150500
---

## Problem
x

## Acceptance criteria
- [ ] x
`

func setupRefineRepo(t *testing.T, sessionID string) string {
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

func TestRefineCmd_RequiresComments(t *testing.T) {
	repoDir := setupRefineRepo(t, "test-refine-no-comments")
	writeItemFile(t, repoDir, "FEAT-401-x.md", refineItemCaptured)
	persistInboxFixture(t, repoDir, "FEAT-401-x.md")

	cmd := newRefineCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"FEAT-401"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected cobra to reject missing --comments, got nil err\nstdout=%s stderr=%s",
			stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.ToLower(err.Error()), "comments") {
		t.Fatalf("error should mention required comments flag, got: %v", err)
	}

	cmd2 := newRefineCmd()
	stdout.Reset()
	stderr.Reset()
	cmd2.SetOut(&stdout)
	cmd2.SetErr(&stderr)
	cmd2.SetArgs([]string{"FEAT-401", "--comments", "   "})
	if err := cmd2.Execute(); err == nil {
		t.Fatalf("whitespace-only --comments should error\nstdout=%s stderr=%s",
			stdout.String(), stderr.String())
	}
}

func TestRefineCmd_FlipsStatus(t *testing.T) {
	repoDir := setupRefineRepo(t, "test-refine-flip")
	writeItemFile(t, repoDir, "FEAT-401-x.md", refineItemCaptured)
	persistInboxFixture(t, repoDir, "FEAT-401-x.md")

	var stdout, stderr bytes.Buffer
	code := runRefineMark(context.Background(), []string{"FEAT-401"}, "tighten the acceptance criteria", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "refined FEAT-401") {
		t.Fatalf("stdout missing 'refined FEAT-401': %s", stdout.String())
	}

	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	itemPath := filepath.Join(root, ".squad", "items", "FEAT-401-x.md")
	raw, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	if !strings.Contains(string(raw), "status: needs-refinement") {
		t.Fatalf("frontmatter status not flipped:\n%s", raw)
	}
	if !strings.Contains(string(raw), "## Reviewer feedback") {
		t.Fatalf("body missing reviewer feedback section:\n%s", raw)
	}
	if !strings.Contains(string(raw), "tighten the acceptance criteria") {
		t.Fatalf("body missing comments text:\n%s", raw)
	}
}

func TestRefineCmd_NoArgs_ListsItems(t *testing.T) {
	repoDir := setupRefineRepo(t, "test-refine-list")
	writeItemFile(t, repoDir, "FEAT-401-x.md", refineItemCaptured)
	writeItemFile(t, repoDir, "FEAT-402-y.md", refineItemSecondCaptured)
	persistInboxFixture(t, repoDir, "FEAT-401-x.md")
	persistInboxFixture(t, repoDir, "FEAT-402-y.md")

	var sink bytes.Buffer
	if code := runRefineMark(context.Background(), []string{"FEAT-401"}, "tighten the acceptance criteria", &sink, &sink); code != 0 {
		t.Fatalf("seed refine exit=%d: %s", code, sink.String())
	}

	var stdout, stderr bytes.Buffer
	code := runRefineList(context.Background(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "FEAT-401") {
		t.Fatalf("stdout missing FEAT-401: %s", out)
	}
	if strings.Contains(out, "FEAT-402") {
		t.Fatalf("stdout should not include captured FEAT-402: %s", out)
	}
}
