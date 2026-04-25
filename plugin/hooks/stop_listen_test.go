package hooks

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStopListen_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "stop_listen.sh")
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

func TestStopListen_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "stop_listen.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}

func TestStopListen_InvokesSquadListen(t *testing.T) {
	stub := writeStubSquad(t, `{"id":"agent-x","item_id":"BUG-1","last_touch":__NOW__}`)
	stubDir := stub[:len(stub)-len("/squad")]
	p := writeFixtureScript(t, "stop_listen.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{
		"PATH=" + stubDir + ":/usr/bin:/bin",
		"SQUAD_LISTEN_ECHO=1",
	}
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "listen") {
		t.Fatalf("expected hook to call `squad listen`, got %q", out)
	}
}
