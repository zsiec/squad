package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeStubPolicy stubs `squad touches policy ...` to echo the supplied JSON
// blob to stdout (and exit 0). All other subcommands no-op. Returns the
// absolute path to the stub binary the hook should invoke via SQUAD_BIN.
func writeStubPolicy(t *testing.T, blob string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "squad")
	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2" in
  "touches policy") printf '%%s\n' '%s' ;;
  *)                exit 0 ;;
esac
`, blob)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return p
}

func TestTouchCheck_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "pre_edit_touch_check.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin",
		`TOOL_INPUT={"file_path":"/tmp/x.go"}`}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
}

func TestTouchCheck_NoConflictPassesThroughCleanBlob(t *testing.T) {
	p := writeFixtureScript(t, "pre_edit_touch_check.sh")
	stub := writeStubPolicy(t, `{"conflict":false}`)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"file_path":"server/main.go"}`, "SQUAD_BIN=" + stub}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"conflict":false`) {
		t.Fatalf("expected stdout to contain {conflict:false}, got %q", out)
	}
}

func TestTouchCheck_AskBlobReachesStdout(t *testing.T) {
	p := writeFixtureScript(t, "pre_edit_touch_check.sh")
	blob := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","additionalContext":"squad: agent-bbbb is editing server/main.go"}}`
	stub := writeStubPolicy(t, blob)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"file_path":"server/main.go"}`, "SQUAD_BIN=" + stub}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"permissionDecision":"ask"`) {
		t.Fatalf("expected ask decision in stdout, got %q", out)
	}
	if !strings.Contains(string(out), "agent-bbbb") {
		t.Fatalf("expected stdout to name agent-bbbb, got %q", out)
	}
}

func TestTouchCheck_DenyBlobReachesStdout(t *testing.T) {
	p := writeFixtureScript(t, "pre_edit_touch_check.sh")
	blob := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","additionalContext":"squad: blocked - agent-cccc is editing go.mod"}}`
	stub := writeStubPolicy(t, blob)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"file_path":"go.mod"}`, "SQUAD_BIN=" + stub}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 (decision is in stdout JSON), got %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"permissionDecision":"deny"`) {
		t.Fatalf("expected deny decision in stdout, got %q", out)
	}
}

func TestTouchCheck_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "pre_edit_touch_check.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
