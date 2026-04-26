package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubagentEvent_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestSubagentEvent_InvokesSquadVerbWithStdin(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	trace := filepath.Join(dir, "trace.txt")
	body := fmt.Sprintf(`#!/bin/sh
{ printf 'argv:'; for a in "$@"; do printf ' %%s' "$a"; done; printf '\n'; cat; } >> %q
exit 0
`, trace)
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "subagent_event.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"SubagentStart","agent_id":"sub-1"}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(trace)
	if err != nil {
		t.Fatalf("read trace: %v", err)
	}
	gotStr := string(got)
	if !strings.Contains(gotStr, "argv: subagent-event") {
		t.Fatalf("did not invoke squad subagent-event:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, `"hook_event_name":"SubagentStart"`) {
		t.Fatalf("stdin not propagated to verb:\n%s", gotStr)
	}
}

func TestSubagentEvent_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "subagent_event.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}
