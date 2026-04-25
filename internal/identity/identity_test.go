package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func clearSessionEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"SQUAD_SESSION_ID", "SQUAD_AGENT",
		"TERM_SESSION_ID", "ITERM_SESSION_ID",
		"TMUX_PANE", "STY", "WT_SESSION",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestSessionKey_PrefersSquadOverride(t *testing.T) {
	clearSessionEnv(t)
	t.Setenv("SQUAD_SESSION_ID", "explicit")
	t.Setenv("TERM_SESSION_ID", "should-be-ignored")
	if got := sessionKey(); got != "explicit" {
		t.Fatalf("got %q want explicit", got)
	}
}

func TestSessionKey_FallsThroughLadder(t *testing.T) {
	clearSessionEnv(t)
	t.Setenv("TERM_SESSION_ID", "term")
	if got := sessionKey(); got != "term" {
		t.Fatalf("got %q want term", got)
	}
}

func TestSessionKey_EmptyWhenNothingSet(t *testing.T) {
	clearSessionEnv(t)
	if got := sessionKey(); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestPersistedAgentID_PerSessionFile(t *testing.T) {
	clearSessionEnv(t)
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "sess-A")
	if err := WritePersistedAgentID("agent-foo"); err != nil {
		t.Fatal(err)
	}
	if got := readPersistedAgentID(); got != "agent-foo" {
		t.Fatalf("read: got %q", got)
	}
	t.Setenv("SQUAD_SESSION_ID", "sess-B")
	if got := readPersistedAgentID(); got != "" {
		t.Fatalf("session-B should not see session-A id, got %q", got)
	}
}

func TestAgentID_HonorsEnvOverride(t *testing.T) {
	clearSessionEnv(t)
	t.Setenv("SQUAD_AGENT", "from-env")
	got, err := AgentID("/tmp/wt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentID_FallsBackToWorktreeBase(t *testing.T) {
	clearSessionEnv(t)
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	got, err := AgentID(filepath.Join("/tmp/projects", "myrepo"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "myrepo" {
		t.Fatalf("got %q", got)
	}
}

func TestAgentID_ErrorWhenNoSignals(t *testing.T) {
	clearSessionEnv(t)
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	if _, err := AgentID(""); err == nil {
		t.Fatal("expected error when SQUAD_AGENT unset and worktree empty")
	}
}
