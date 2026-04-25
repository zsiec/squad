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

// QA r6-E F4: a fresh session re-using an existing agent's id used to
// silently re-point the agents row at the new session, conflating
// identities. The guard now refuses unless --force is passed.
func TestRegister_RefusesHijackingExistingAgent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)

	// Session 1: register agent-foo from worktree-A.
	t.Setenv("SQUAD_SESSION_ID", "session-1")
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"register", "--as", "agent-foo", "--no-repo-check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("session-1 register: %v\n%s", err, out.String())
	}

	// Session 2: same id, different session. Should refuse.
	t.Setenv("SQUAD_SESSION_ID", "session-2")
	root2 := newRootCmd()
	var out2 bytes.Buffer
	root2.SetOut(&out2)
	root2.SetErr(&out2)
	root2.SetArgs([]string{"register", "--as", "agent-foo", "--no-repo-check"})
	err := root2.Execute()
	if err == nil {
		t.Fatalf("expected refusal for hijack attempt, got success: %s", out2.String())
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("error should mention 'already registered', got: %v", err)
	}

	// With --force, the same call should succeed.
	root3 := newRootCmd()
	var out3 bytes.Buffer
	root3.SetOut(&out3)
	root3.SetErr(&out3)
	root3.SetArgs([]string{"register", "--as", "agent-foo", "--force", "--no-repo-check"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("--force register failed: %v\n%s", err, out3.String())
	}
}

func TestRegister_NoFlags_AutoDerivesIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-derive-1")
	t.Setenv("SQUAD_AGENT", "")
	t.Chdir(t.TempDir())

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"register", "--no-repo-check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "registered agent-") {
		t.Fatalf("want `registered agent-XXXX`, got %q", got)
	}
}

func TestRegister_ZeroArg_StableAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "stable-session")
	t.Setenv("SQUAD_AGENT", "")
	t.Chdir(t.TempDir())

	first := newRootCmd()
	var out1 bytes.Buffer
	first.SetOut(&out1)
	first.SetErr(&out1)
	first.SetArgs([]string{"register", "--no-repo-check"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first execute: %v\n%s", err, out1.String())
	}

	second := newRootCmd()
	var out2 bytes.Buffer
	second.SetOut(&out2)
	second.SetErr(&out2)
	second.SetArgs([]string{"register", "--no-repo-check"})
	if err := second.Execute(); err != nil {
		t.Fatalf("second execute: %v\n%s", err, out2.String())
	}

	if strings.TrimSpace(out1.String()) != strings.TrimSpace(out2.String()) {
		t.Fatalf("identity not stable across runs: %q vs %q",
			out1.String(), out2.String())
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
