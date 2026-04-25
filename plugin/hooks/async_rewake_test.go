package hooks

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAsyncRewake_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "async_rewake.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent, got %q", out)
	}
}

func TestAsyncRewake_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "async_rewake.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}
