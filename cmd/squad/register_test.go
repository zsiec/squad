package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

func TestRegister_NoRepoCheck_WritesAgentRow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-1")
	t.Setenv("SQUAD_AGENT", "")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"register", "--as", "agent-foo", "--name", "Agent Foo", "--no-repo-check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "registered agent-foo") {
		t.Fatalf("expected `registered agent-foo`, got %q", out.String())
	}

	db, err := store.Open(filepath.Join(dir, "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var gotID, gotName, gotRepo string
	if err := db.QueryRowContext(context.Background(),
		`SELECT id, display_name, repo_id FROM agents WHERE id=?`, "agent-foo",
	).Scan(&gotID, &gotName, &gotRepo); err != nil {
		t.Fatal(err)
	}
	if gotID != "agent-foo" || gotName != "Agent Foo" || gotRepo != "_unscoped" {
		t.Fatalf("got id=%q name=%q repo=%q", gotID, gotName, gotRepo)
	}
}

func TestRegister_RejectsOversizedAs(t *testing.T) {
	t.Setenv("SQUAD_HOME", t.TempDir())
	t.Setenv("SQUAD_SESSION_ID", "test-session-oversized")
	huge := strings.Repeat("x", MaxAgentIDLen+1)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"register", "--as", huge, "--no-repo-check"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for oversized --as, got nil")
	}
}

func TestRegister_RejectsOversizedName(t *testing.T) {
	t.Setenv("SQUAD_HOME", t.TempDir())
	t.Setenv("SQUAD_SESSION_ID", "test-session-oversized-name")
	huge := strings.Repeat("x", MaxDisplayNameLen+1)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"register", "--as", "agent-ok", "--name", huge, "--no-repo-check"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for oversized --name, got nil")
	}
}

func TestRegister_NoFlags_RequiresInit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-2")
	t.Setenv("SQUAD_AGENT", "")
	t.Chdir(t.TempDir())

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"register", "--as", "agent-bar"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no .squad/config.yaml and --no-repo-check absent")
	}
	if !strings.Contains(err.Error(), "squad init") {
		t.Fatalf("error should mention `squad init`: %v", err)
	}
}
