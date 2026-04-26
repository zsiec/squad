package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoopPreBashTick_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "loop_pre_bash_tick.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestLoopPreBashTick_SilentWhenSquadMissing(t *testing.T) {
	p := writeFixtureScript(t, "loop_pre_bash_tick.sh")
	cmd := exec.Command("/bin/sh", p)
	// Empty PATH so `command -v squad` fails.
	cmd.Env = []string{"PATH="}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent without squad, got %q", out)
	}
}

func TestLoopPreBashTick_EmitsContextWithJQ(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	body := `#!/bin/sh
case "$1" in
  tick) printf '%s\n' 'pending: 2 mentions' ;;
  *)    exit 0 ;;
esac
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "loop_pre_bash_tick.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"additionalContext"`) {
		t.Fatalf("expected hookSpecificOutput envelope; got: %s", out)
	}
	if !strings.Contains(string(out), "pending: 2 mentions") {
		t.Fatalf("expected tick output to be embedded; got: %s", out)
	}
}

// TestLoopPreBashTick_FallsBackWithoutJQ verifies the script does not crash
// under set -euo pipefail when jq is not on PATH. Previously the script
// piped unconditionally to `jq -Rs .`, which exited non-zero under pipefail
// on hosts without jq and aborted the hook.
func TestLoopPreBashTick_FallsBackWithoutJQ(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	body := `#!/bin/sh
case "$1" in
  tick) printf '%s\n' 'plain "tick" output with backslash \ here' ;;
  *)    exit 0 ;;
esac
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	// Build a sandbox PATH that contains the system shell utilities (sed,
	// awk, printf, etc.) plus our stub squad — but NOT jq. We resolve the
	// real binaries from the test process's PATH and symlink them into the
	// sandbox so jq stays out.
	for _, bin := range []string{"sed", "awk", "head", "grep", "sh", "command"} {
		realPath, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		if err := os.Symlink(realPath, filepath.Join(dir, bin)); err != nil {
			// `command` is a shell builtin; symlinking is best-effort.
			continue
		}
	}
	hookPath := writeFixtureScript(t, "loop_pre_bash_tick.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = []string{"PATH=" + dir}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook should tolerate missing jq, got exit %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"additionalContext"`) {
		t.Fatalf("expected hookSpecificOutput envelope from fallback; got: %s", out)
	}
	// The backslash and embedded quote must be escaped in the JSON string.
	got := string(out)
	if !strings.Contains(got, `\\`) {
		t.Errorf("expected backslash to be escaped as \\\\ in JSON string; got: %s", got)
	}
	if !strings.Contains(got, `\"tick\"`) {
		t.Errorf("expected embedded quotes to be escaped as \\\"; got: %s", got)
	}
}

func TestLoopPreBashTick_DashLintClean(t *testing.T) {
	requireDash(t)
	// loop_pre_bash_tick.sh starts with #!/usr/bin/env bash; run dash -n on
	// the fixture path to check the bash script is at least dash-syntactically
	// valid (the script avoids bash-only constructs).
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "loop_pre_bash_tick.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
