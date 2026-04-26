package hooks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fakeSquad(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "squad")
	body := `#!/bin/sh
case "$1 $2" in
    "learning triviality-check") echo "` + mode + `" ;;
    "learning list") echo "" ;;
    *) echo "{}" ;;
esac
exit 0
`
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func runHook(t *testing.T, name string, env map[string]string) string {
	t.Helper()
	script, err := FS.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	tmp := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(tmp, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", tmp)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("hook exit: %v", err)
	}
	return stdout.String()
}

func TestStopLearningPrompt_TrivialDiffSilent(t *testing.T) {
	bin := fakeSquad(t, "trivial")
	out := runHook(t, "stop_learning_prompt.sh", map[string]string{"SQUAD_BIN": bin})
	if strings.TrimSpace(out) != "" {
		t.Errorf("trivial diff should produce no output, got %q", out)
	}
}

func TestStopLearningPrompt_NonTrivialDiffPrintsPrompt(t *testing.T) {
	bin := fakeSquad(t, "non-trivial")
	out := runHook(t, "stop_learning_prompt.sh", map[string]string{"SQUAD_BIN": bin})
	if !strings.Contains(out, "squad learning propose") {
		t.Errorf("non-trivial diff should suggest propose, got %q", out)
	}
}

func TestStopLearningPrompt_DisabledByEnv(t *testing.T) {
	bin := fakeSquad(t, "non-trivial")
	out := runHook(t, "stop_learning_prompt.sh", map[string]string{
		"SQUAD_BIN": bin, "SQUAD_NO_HOOKS": "1",
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("SQUAD_NO_HOOKS=1 should silence the hook, got %q", out)
	}
}
