package hooks

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSQUAD_NO_HOOKS_DisablesEveryHook(t *testing.T) {
	for _, h := range All {
		t.Run(h.Name, func(t *testing.T) {
			p := writeFixtureScript(t, h.Filename)
			// Run via the script's own shebang. Some hooks declare
			// #!/usr/bin/env bash (e.g. loop_pre_bash_tick uses
			// `set -o pipefail` which isn't in POSIX sh / dash); forcing
			// /bin/sh makes the test fail on Linux runners where /bin/sh
			// is dash.
			cmd := exec.Command(p)
			cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin",
				`TOOL_INPUT={"command":"git commit -m foo","file_path":"x.go"}`}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s: expected exit 0, got %v: %s", h.Filename, err, out)
			}
			if strings.TrimSpace(string(out)) != "" {
				t.Fatalf("%s: expected silent, got %q", h.Filename, out)
			}
		})
	}
}
