package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserPromptSubmit_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "user_prompt_submit.sh")
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

func TestUserPromptSubmit_NoOpWhenSquadMissing(t *testing.T) {
	p := writeFixtureScript(t, "user_prompt_submit.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "SQUAD_BIN=squad-not-installed"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook should not fail when squad missing: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent fallback when squad missing, got %q", out)
	}
}

func TestUserPromptSubmit_ForwardsMailboxEnvelope(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	// `squad mailbox --format additional-context` is responsible for
	// emitting the hookSpecificOutput envelope itself; the hook is now a
	// thin shim that just lets its stdout flow through to Claude Code.
	body := `#!/bin/sh
case "$1" in
  mailbox) printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"[squad inbox]\nalice asks about BUG-1"}}' ;;
  *) exit 0 ;;
esac
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	hookPath := writeFixtureScript(t, "user_prompt_submit.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, `"hookEventName":"UserPromptSubmit"`) {
		t.Fatalf("expected hookSpecificOutput envelope; got: %s", got)
	}
	if !strings.Contains(got, "additionalContext") {
		t.Fatalf("expected additionalContext field; got: %s", got)
	}
	if !strings.Contains(got, "alice") || !strings.Contains(got, "BUG-1") {
		t.Fatalf("expected mention body to flow into context; got: %s", got)
	}
}

func TestUserPromptSubmit_QuietWhenMailboxSilent(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	body := `#!/bin/sh
exit 0
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	hookPath := writeFixtureScript(t, "user_prompt_submit.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent when mailbox emits nothing, got: %q", out)
	}
}

func TestUserPromptSubmit_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "user_prompt_submit.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}

func TestUserPromptSubmit_UsesMailboxCommand(t *testing.T) {
	stub := writeStubSquad(t, `{"id":"agent-x"}`)
	stubDir := stub[:len(stub)-len("/squad")]
	p := writeFixtureScript(t, "user_prompt_submit.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{
		"PATH=" + stubDir + ":/usr/bin:/bin",
		"SQUAD_HOOK_TRACE=1",
	}
	out, _ := cmd.CombinedOutput()
	if strings.Contains(string(out), "tick --json") {
		t.Fatalf("user_prompt_submit must not call `tick`; should use `mailbox` now: %q", out)
	}
}
