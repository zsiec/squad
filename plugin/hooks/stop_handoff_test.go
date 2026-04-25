package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeStubStopHandoff(t *testing.T, whoami, logPath string) string {
	t.Helper()
	now := strconv.FormatInt(time.Now().Unix(), 10)
	body := strings.ReplaceAll(whoami, "__NOW__", now)
	dir := t.TempDir()
	p := filepath.Join(dir, "squad")
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s ' "$@" >> %q
printf '\n' >> %q
case "$1" in
  whoami) printf '%%s\n' '%s' ;;
  *)      exit 0 ;;
esac
`, logPath, logPath, body)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return p
}

func TestStopHandoff_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "stop_handoff.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
}

func TestStopHandoff_NoClaim(t *testing.T) {
	p := writeFixtureScript(t, "stop_handoff.sh")
	log := filepath.Join(t.TempDir(), "log")
	stub := writeStubStopHandoff(t, `{"id":"agent-aaaa"}`, log)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "SQUAD_BIN=" + stub}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
}

func TestStopHandoff_RunsWhenStale(t *testing.T) {
	p := writeFixtureScript(t, "stop_handoff.sh")
	log := filepath.Join(t.TempDir(), "log")
	stub := writeStubStopHandoff(t, `{"id":"agent-aaaa","item_id":"FEAT-1","last_touch":1}`, log)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "SQUAD_BIN=" + stub}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	body, _ := os.ReadFile(log)
	if !strings.Contains(string(body), "handoff") {
		t.Fatalf("expected stub to receive handoff, got %q", body)
	}
}

func TestStopHandoff_SkipsWhenRecent(t *testing.T) {
	p := writeFixtureScript(t, "stop_handoff.sh")
	log := filepath.Join(t.TempDir(), "log")
	stub := writeStubStopHandoff(t, `{"id":"agent-aaaa","item_id":"FEAT-1","last_touch":__NOW__}`, log)
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "SQUAD_BIN=" + stub}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
	body, _ := os.ReadFile(log)
	if strings.Contains(string(body), "handoff") {
		t.Fatalf("expected no handoff when recent, got %q", body)
	}
}

func TestStopHandoff_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "stop_handoff.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
