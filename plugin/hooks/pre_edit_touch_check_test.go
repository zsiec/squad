package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeStubTouches(t *testing.T, j string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "squad")
	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2" in
  "touches list-others") printf '%%s\n' '%s' ;;
  "whoami "*)            printf '%%s\n' '{"id":"agent-aaaa"}' ;;
  *)                     exit 0 ;;
esac
`, j)
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

func TestTouchCheck_SilentNoConflict(t *testing.T) {
	p := writeFixtureScript(t, "pre_edit_touch_check.sh")
	stub := writeStubTouches(t, "[]")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"file_path":"server/main.go"}`, "SQUAD_BIN=" + stub}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestTouchCheck_WarnsOnConflict(t *testing.T) {
	p := writeFixtureScript(t, "pre_edit_touch_check.sh")
	stub := writeStubTouches(t, `[{"agent_id":"agent-bbbb","repo":"my-repo","path":"server/main.go"}]`)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"file_path":"server/main.go"}`, "SQUAD_BIN=" + stub}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 (warn-only), got %v: %s", err, out)
	}
	if !strings.Contains(string(out), "agent-bbbb") {
		t.Fatalf("expected stderr to name agent-bbbb, got %q", out)
	}
	if !strings.Contains(string(out), "squad knock") {
		t.Fatalf("expected stderr to suggest knock, got %q", out)
	}
}

func TestTouchCheck_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "pre_edit_touch_check.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
