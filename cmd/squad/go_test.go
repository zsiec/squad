package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func writeItemFile(t *testing.T, repoDir, name, body string) {
	t.Helper()
	p := filepath.Join(repoDir, ".squad", "items", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGoCmd_Exists(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Use == "go" {
			return
		}
	}
	t.Fatal("squad go command not registered on root")
}

func TestGoCmd_HelpMentionsOrchestration(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	body := out.String()
	for _, want := range []string{"register", "claim", "mailbox"} {
		if !strings.Contains(strings.ToLower(body), want) {
			t.Errorf("help should mention %q, got: %s", want, body)
		}
	}
}

func TestGoCmd_InitsWhenSquadDirAbsent(t *testing.T) {
	repo := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-init-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repo)
	t.Chdir(repo)

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	_ = root.Execute()

	for _, rel := range []string{
		".squad/config.yaml",
		".squad/STATUS.md",
		".squad/items/EXAMPLE-001-try-the-loop.md",
		"AGENTS.md",
	} {
		if _, err := os.Stat(filepath.Join(repo, rel)); err != nil {
			t.Errorf("squad go did not init %s: %v", rel, err)
		}
	}
}

func TestGoCmd_DoesNotReinitWhenSquadDirPresent(t *testing.T) {
	repo := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-init-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repo)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repo})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(repo, ".squad", "config.yaml")
	mtimeBefore, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(repo)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	_ = root.Execute()

	mtimeAfter, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !mtimeAfter.ModTime().Equal(mtimeBefore.ModTime()) {
		t.Fatal("squad go re-wrote .squad/config.yaml on second run")
	}
}

func TestGoCmd_RegistersAgentWhenAbsent(t *testing.T) {
	repo := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-reg-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repo)
	t.Chdir(repo)

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	_ = root.Execute()

	db, err := store.Open(filepath.Join(state, "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agents WHERE id LIKE 'agent-%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 agent row, got %d", n)
	}
}

func TestGoCmd_DoesNotReregisterOnSecondRun(t *testing.T) {
	repo := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-reg-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repo)
	t.Chdir(repo)

	for i := 0; i < 2; i++ {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"go"})
		_ = root.Execute()
	}

	db, err := store.Open(filepath.Join(state, "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agents`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 agent row across two runs, got %d", n)
	}
}

func TestGoCmd_ClaimsTopReadyItem(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-claim-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))
	writeItemFile(t, repoDir, "FEAT-001-pick-me.md",
		"---\nid: FEAT-001\ntitle: pick me\ntype: feature\npriority: P0\nstatus: open\nestimate: 1h\n---\n\n## Acceptance criteria\n- [ ] do the thing\n")
	writeItemFile(t, repoDir, "FEAT-002-skip-me.md",
		"---\nid: FEAT-002\ntitle: skip me\ntype: feature\npriority: P2\nstatus: open\nestimate: 1h\n---\n")

	t.Chdir(repoDir)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nout=%s", err, out.String())
	}

	if !strings.Contains(out.String(), "FEAT-001") {
		t.Fatalf("expected FEAT-001 to be claimed; got %s", out.String())
	}

	db, err := store.Open(filepath.Join(state, "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var item string
	if err := db.QueryRowContext(context.Background(),
		`SELECT item_id FROM claims`).Scan(&item); err != nil {
		t.Fatalf("expected exactly one claim row, got: %v", err)
	}
	if item != "FEAT-001" {
		t.Fatalf("want FEAT-001 claimed, got %q", item)
	}
	_ = items.Walk
	_ = repo.Discover
}

func TestGoCmd_NoReadyItemsIsNotAnError(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-claim-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))

	t.Chdir(repoDir)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	if err := root.Execute(); err != nil {
		t.Fatalf("squad go should succeed with no items: %v\n%s",
			err, out.String())
	}
	if !strings.Contains(strings.ToLower(out.String()), "no ready") {
		t.Fatalf("expected 'no ready' message, got %s", out.String())
	}
}

func TestGoCmd_PrintsAcceptanceCriteria(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-ac-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))
	writeItemFile(t, repoDir, "FEAT-010-with-ac.md",
		`---
id: FEAT-010
title: with ac
type: feature
priority: P0
status: open
estimate: 1h
---

## Acceptance criteria
- [ ] first specific testable thing
- [ ] second specific testable thing
- [ ] third specific testable thing
`)

	t.Chdir(repoDir)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	body := out.String()
	for _, want := range []string{
		"first specific testable thing",
		"second specific testable thing",
		"third specific testable thing",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("AC line %q missing from output:\n%s", want, body)
		}
	}
}

func TestGoCmd_FlushesMailboxAtEnd(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-flush-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))
	writeItemFile(t, repoDir, "FEAT-100-flush.md",
		"---\nid: FEAT-100\ntitle: flush\ntype: feature\npriority: P0\nstatus: open\nestimate: 1h\n---\n\n## Acceptance criteria\n- [ ] x\n")

	t.Chdir(repoDir)

	first2 := newRootCmd()
	var out1 bytes.Buffer
	first2.SetOut(&out1)
	first2.SetErr(&out1)
	first2.SetArgs([]string{"go"})
	if err := first2.Execute(); err != nil {
		t.Fatalf("first run: %v\n%s", err, out1.String())
	}

	dbPath, _ := store.DBPath()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var repoID string
	if err := db.QueryRowContext(context.Background(),
		`SELECT id FROM repos LIMIT 1`).Scan(&repoID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO agents (id, repo_id, display_name, started_at, last_tick_at, status)
		VALUES (?, ?, ?, ?, ?, 'active')
		ON CONFLICT(id) DO NOTHING`,
		"agent-other", repoID, "Other", 1, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, 'global', 'fyi', 'hello-from-other', '', 'normal')`,
		repoID, time.Now().Unix(), "agent-other"); err != nil {
		t.Fatal(err)
	}

	second := newRootCmd()
	var out2 bytes.Buffer
	second.SetOut(&out2)
	second.SetErr(&out2)
	second.SetArgs([]string{"go"})
	if err := second.Execute(); err != nil {
		t.Fatalf("second run: %v\n%s", err, out2.String())
	}

	if !strings.Contains(out2.String(), "hello-from-other") {
		t.Fatalf("squad go did not flush mailbox; output=%s", out2.String())
	}
}

func TestGoCmd_IdempotentTwoRunsOneClaim(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-idem")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))
	writeItemFile(t, repoDir, "FEAT-A.md",
		"---\nid: FEAT-100\ntitle: a\ntype: feature\npriority: P0\nstatus: open\nestimate: 1h\n---\n")
	writeItemFile(t, repoDir, "FEAT-B.md",
		"---\nid: FEAT-200\ntitle: b\ntype: feature\npriority: P0\nstatus: open\nestimate: 1h\n---\n")

	t.Chdir(repoDir)
	for i := 0; i < 2; i++ {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"go"})
		if err := root.Execute(); err != nil {
			t.Fatalf("run %d: %v\n%s", i, err, out.String())
		}
	}

	dbPath, _ := store.DBPath()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM claims`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 claim across two squad-go runs, got %d", n)
	}
}

func TestGoCmd_IdentityStableAcrossRuns(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "stable-session-id")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}

	collectAgents := func() []string {
		dbPath, _ := store.DBPath()
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		rows, err := db.QueryContext(context.Background(),
			`SELECT id FROM agents ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			got = append(got, s)
		}
		return got
	}

	t.Chdir(repoDir)
	for i := 0; i < 3; i++ {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"go"})
		_ = root.Execute()
	}

	got := collectAgents()
	if len(got) != 1 {
		t.Fatalf("want exactly 1 agent across 3 runs of the same session, got %v", got)
	}
}
