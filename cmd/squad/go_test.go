package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

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
