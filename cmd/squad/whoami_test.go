package main

import (
	"bytes"
	"context"
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

func TestWhoami_RendersCapabilities(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-whoami-cap")
	t.Setenv("SQUAD_AGENT", "")

	reg := newRootCmd()
	var regOut bytes.Buffer
	reg.SetOut(&regOut)
	reg.SetErr(&regOut)
	reg.SetArgs([]string{"register", "--as", "agent-cap", "--no-repo-check",
		"--capability", "go", "--capability", "sql"})
	if err := reg.Execute(); err != nil {
		t.Fatalf("register: %v\n%s", err, regOut.String())
	}

	who := newRootCmd()
	var out bytes.Buffer
	who.SetOut(&out)
	who.SetErr(&out)
	who.SetArgs([]string{"whoami", "--json"})
	if err := who.Execute(); err != nil {
		t.Fatalf("whoami: %v\n%s", err, out.String())
	}
	body := out.String()
	for _, want := range []string{`"capabilities"`, `"go"`, `"sql"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in: %s", want, body)
		}
	}
}

func TestWhoami_EmptyCapabilitiesShowsNone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SQUAD_HOME", dir)
	t.Setenv("SQUAD_SESSION_ID", "test-whoami-no-cap")
	t.Setenv("SQUAD_AGENT", "")

	reg := newRootCmd()
	var regOut bytes.Buffer
	reg.SetOut(&regOut)
	reg.SetErr(&regOut)
	reg.SetArgs([]string{"register", "--as", "agent-bare", "--no-repo-check"})
	if err := reg.Execute(); err != nil {
		t.Fatalf("register: %v\n%s", err, regOut.String())
	}

	who := newRootCmd()
	var out bytes.Buffer
	who.SetOut(&out)
	who.SetErr(&out)
	who.SetArgs([]string{"whoami", "--verbose"})
	if err := who.Execute(); err != nil {
		t.Fatalf("whoami: %v\n%s", err, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "agent-bare") {
		t.Errorf("missing agent id in: %s", body)
	}
	if !strings.Contains(body, "(none)") {
		t.Errorf("expected '(none)' rendering for empty capability set, got: %s", body)
	}
}

func TestWhoami_WithExistingDB(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	withDB, err := Whoami(ctx, WhoamiArgs{DB: env.DB})
	if err != nil {
		t.Fatalf("Whoami with DB: %v", err)
	}
	withoutDB, err := Whoami(ctx, WhoamiArgs{})
	if err != nil {
		t.Fatalf("Whoami without DB: %v", err)
	}
	if withDB.AgentID != withoutDB.AgentID {
		t.Fatalf("AgentID drift: with=%q without=%q", withDB.AgentID, withoutDB.AgentID)
	}
	if withDB.LastTickAt != withoutDB.LastTickAt {
		t.Fatalf("LastTickAt drift: with=%d without=%d", withDB.LastTickAt, withoutDB.LastTickAt)
	}
	if withDB.ItemID != withoutDB.ItemID {
		t.Fatalf("ItemID drift: with=%q without=%q", withDB.ItemID, withoutDB.ItemID)
	}
	if withDB.Intent != withoutDB.Intent {
		t.Fatalf("Intent drift: with=%q without=%q", withDB.Intent, withoutDB.Intent)
	}
	if withDB.LastTouch != withoutDB.LastTouch {
		t.Fatalf("LastTouch drift: with=%d without=%d", withDB.LastTouch, withoutDB.LastTouch)
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
