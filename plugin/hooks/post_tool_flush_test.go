package hooks

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPostToolFlush_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "post_tool_flush.sh")
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

func TestPostToolFlush_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "post_tool_flush.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}
