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

func TestUserPromptSubmit_EmitsAdditionalContextWhenMentionsPending(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	// Stub `squad tick --json` with a digest carrying a single mention.
	// The hook should turn that into a hookSpecificOutput JSON envelope.
	body := `#!/bin/sh
case "$1 $2" in
  "tick --json") printf '%s\n' '{"agent":"agent-a","mentions":[{"agent":"alice","kind":"ask","thread":"global","body":"can you review BUG-1?","ts":1000}],"knocks":[],"handoffs":[],"your_threads":[],"global":[],"lost_claims":[]}' ;;
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

func TestUserPromptSubmit_QuietWhenNothingPending(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	body := `#!/bin/sh
case "$1 $2" in
  "tick --json") printf '%s\n' '{"agent":"agent-a","mentions":[],"knocks":[],"handoffs":[],"your_threads":[],"global":[],"lost_claims":[]}' ;;
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
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent when digest is empty, got: %q", out)
	}
}

func TestUserPromptSubmit_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "user_prompt_submit.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}
