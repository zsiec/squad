package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T, body string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "t@e.com"},
		{"config", "user.name", "t"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := exec.Command("git", "add", "f.txt")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	return dir
}

func TestPMTraces_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m foo"}`}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
}

func TestPMTraces_PassesClean(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	repo := setupGitRepo(t, "")
	cmd := exec.Command("/bin/sh", p)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m 'feat: add cool thing'"}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0, got %v: %s", err, out)
	}
}

func TestPMTraces_FailsOnIDInMessage(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	repo := setupGitRepo(t, "")
	cmd := exec.Command("/bin/sh", p)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m 'fix: address BUG-123'"}`)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for BUG-123, got success: %s", out)
	}
	if !strings.Contains(string(out), "BUG-123") {
		t.Fatalf("expected stderr to mention BUG-123, got %q", out)
	}
}

func TestPMTraces_FailsOnIDInDiff(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	repo := setupGitRepo(t, "// note: see FEAT-42 for context\n")
	cmd := exec.Command("/bin/sh", p)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin",
		`TOOL_INPUT={"command":"git commit -m 'feat: clean'"}`)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for FEAT-42 in diff, got success: %s", out)
	}
	if !strings.Contains(string(out), "FEAT-42") {
		t.Fatalf("expected stderr to mention FEAT-42, got %q", out)
	}
}

func TestPMTraces_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "pre_commit_pm_traces.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
