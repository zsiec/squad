package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/store"
)

// Regression for BUG-017: SessionStart hook registers via `--no-repo-check`,
// which writes `repo_id="_unscoped"`. `squad go` already upgrades on its
// orchestration path, but agents that drive the lifecycle from MCP/Claude
// tools (claim, chat, etc.) without ever invoking `squad go` stayed unscoped
// — invisible to `squad who` and the dashboard. Any repo-scoped CLI command
// must upgrade the row on first use.
func TestBootClaimContext_UpgradesUnscopedAgent(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-bug-017-claim")
	t.Setenv("SQUAD_AGENT", "")
	// Disable the cobra post-run hygiene hook so register's epilogue
	// doesn't itself drive the upgrade — the test would otherwise pass
	// vacuously without exercising bootClaimContext directly.
	t.Setenv("SQUAD_NO_HYGIENE", "1")
	gitInitDir(t, repoDir)

	initCmd := newInitCmd()
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Chdir(repoDir)

	regCmd := newRootCmd()
	regCmd.SetArgs([]string{"register", "--no-repo-check"})
	regCmd.SetOut(&bytes.Buffer{})
	regCmd.SetErr(&bytes.Buffer{})
	if err := regCmd.Execute(); err != nil {
		t.Fatalf("register --no-repo-check: %v", err)
	}

	dbPath := filepath.Join(state, "global.db")
	readRepoID := func() string {
		t.Helper()
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var got string
		if err := db.QueryRowContext(context.Background(),
			`SELECT repo_id FROM agents WHERE id LIKE 'agent-%'`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if pre := readRepoID(); pre != "_unscoped" {
		t.Fatalf("setup precondition: expected _unscoped after --no-repo-check, got %q", pre)
	}

	bc, err := bootClaimContext(context.Background())
	if err != nil {
		t.Fatalf("bootClaimContext: %v", err)
	}
	bc.Close()

	if got := readRepoID(); got == "_unscoped" {
		t.Fatalf("agent row still unscoped after bootClaimContext; want repo_id upgraded")
	}
}
