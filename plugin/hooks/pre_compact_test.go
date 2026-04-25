package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreCompact_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "pre_compact.sh")
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

func TestPreCompact_NoOpWhenWhoamiEmpty(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "pre_compact.sh")
	cmd := exec.Command("/bin/sh", hookPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PATH=%s:/usr/bin:/bin", dir),
		"SQUAD_BIN="+stub,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exit %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("expected silent without an agent, got: %q", out)
	}
}

func TestPreCompact_EmitsClaimAndRecentChat(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "squad")
	body := `#!/bin/sh
case "$1 $2" in
  "whoami --json") printf '%s\n' '{"id":"agent-blue","item_id":"FEAT-007","intent":"wire export"}' ;;
  "tail --since")  printf '%s\n' '14:02 agent-blue (milestone, #FEAT-007): AC1 green' ;;
  *)               exit 0 ;;
esac
`
	if err := os.WriteFile(stub, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := writeFixtureScript(t, "pre_compact.sh")
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
	if !strings.Contains(got, `"hookEventName":"PreCompact"`) {
		t.Fatalf("expected PreCompact envelope; got: %s", got)
	}
	for _, want := range []string{"agent-blue", "FEAT-007", "wire export"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in additionalContext; got: %s", want, got)
		}
	}
}

func TestPreCompact_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "pre_compact.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n failed: %v\n%s", err, out)
	}
}
