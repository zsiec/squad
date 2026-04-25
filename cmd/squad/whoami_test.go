package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWhoami_AfterRegister_PrintsAgentID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-whoami")
	t.Setenv("SQUAD_AGENT", "")

	reg := newRootCmd()
	var regOut bytes.Buffer
	reg.SetOut(&regOut)
	reg.SetErr(&regOut)
	reg.SetArgs([]string{"register", "--as", "agent-zed", "--no-repo-check"})
	if err := reg.Execute(); err != nil {
		t.Fatalf("register: %v\n%s", err, regOut.String())
	}

	who := newRootCmd()
	var out bytes.Buffer
	who.SetOut(&out)
	who.SetErr(&out)
	who.SetArgs([]string{"whoami"})
	if err := who.Execute(); err != nil {
		t.Fatalf("whoami: %v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "agent-zed" {
		t.Fatalf("got %q want agent-zed", got)
	}
}

func TestWhoami_NoIdentity_DerivesAgentSuffix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-session-whoami-empty")
	t.Setenv("SQUAD_AGENT", "")
	wt := t.TempDir()
	t.Chdir(wt)

	who := newRootCmd()
	var out bytes.Buffer
	who.SetOut(&out)
	who.SetErr(&out)
	who.SetArgs([]string{"whoami"})
	if err := who.Execute(); err != nil {
		t.Fatalf("expected derived id, got error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.HasPrefix(got, "agent-") {
		t.Fatalf("got %q, want agent- prefix", got)
	}
	if len(got) != len("agent-")+4 {
		t.Fatalf("got %q, want agent-XXXX (4 hex chars)", got)
	}
}
