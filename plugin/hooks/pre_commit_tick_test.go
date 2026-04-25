package hooks

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPreCommitTick_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_tick.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m foo"}`}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
}

func TestPreCommitTick_PassesNonGitCommit(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_tick.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"ls -la"}`, "SQUAD_BIN=/bin/false"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0 for non-commit, got %v: %s", err, out)
	}
}

func TestPreCommitTick_FailsWhenStale(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_tick.sh")
	stub := writeStubSquad(t, `{"id":"agent-aaaa","last_tick_at":1}`)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m foo"}`, "SQUAD_BIN=" + stub}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit, got success: %s", out)
	}
	if !strings.Contains(string(out), "squad tick") {
		t.Fatalf("expected stderr to mention `squad tick`, got %q", out)
	}
}

func TestPreCommitTick_PassesWhenRecent(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_tick.sh")
	stub := writeStubSquad(t, `{"id":"agent-aaaa","last_tick_at":__NOW__}`)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m foo"}`, "SQUAD_BIN=" + stub}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0 when recent, got %v: %s", err, out)
	}
}

func TestPreCommitTick_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "pre_commit_tick.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
