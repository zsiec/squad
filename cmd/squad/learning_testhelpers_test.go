package main

import (
	"bytes"
	"testing"
)

// setupSquadRepo creates a tmp git repo, runs `squad init --yes --dir <repo>`,
// sets SQUAD_HOME / SQUAD_SESSION_ID / SQUAD_AGENT to test-safe values, and
// returns the repo root. Caller is responsible for `t.Chdir(root)` if needed.
func setupSquadRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-session")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return repoDir
}
