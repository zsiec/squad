package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepoWithStagedFile inits a git repo and stages a file with the
// supplied name + body, returning the repo dir.
func setupRepoWithStagedFile(t *testing.T, name, body string) string {
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
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "add", name)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	return dir
}

func TestAgentsMdHook_NoOpWhenDisabled(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_agents_md.sh")
	cmd := exec.Command("/bin/sh", p)
	cmd.Env = []string{"SQUAD_NO_HOOKS=1", "PATH=/usr/bin:/bin"}
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git commit -m foo"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected exit 0 with SQUAD_NO_HOOKS=1, got %v: %s", err, out)
	}
}

// TestAgentsMdHook_SkipsCommitsThatDoNotTouchAgentsMd verifies the hook
// stays out of the way when AGENTS.md is not in the staged set — a
// commit touching only test files should never be blocked by drift in
// AGENTS.md regardless of whether the binary is on PATH.
func TestAgentsMdHook_SkipsCommitsThatDoNotTouchAgentsMd(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_agents_md.sh")
	repo := setupRepoWithStagedFile(t, "f.txt", "unrelated content\n")
	cmd := exec.Command("/bin/sh", p)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin")
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git commit -m 'feat: add f'"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit without AGENTS.md should pass; got %v: %s", err, out)
	}
}

// TestAgentsMdHook_SkipsNonBashTools mirrors the pm-traces shape: only
// Bash commit invocations are scanned. An Edit event must pass through
// silently even when its content sits in a repo with AGENTS.md.
func TestAgentsMdHook_SkipsNonBashTools(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_agents_md.sh")
	repo := setupRepoWithStagedFile(t, "AGENTS.md", "# stale\n")
	cmd := exec.Command("/bin/sh", p)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin")
	cmd.Stdin = strings.NewReader(`{"tool_name":"Edit","tool_input":{"file_path":"AGENTS.md","new_string":"x"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Edit event must pass through; got %v: %s", err, out)
	}
}

// TestAgentsMdHook_SkipsWhenSquadBinaryAbsent confirms a misconfigured
// plugin (squad not on PATH) does not block commits — drift checking
// is best-effort, exits 0 on the bypass branch.
func TestAgentsMdHook_SkipsWhenSquadBinaryAbsent(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_agents_md.sh")
	repo := setupRepoWithStagedFile(t, "AGENTS.md", "# any\n")
	cmd := exec.Command("/bin/sh", p)
	cmd.Dir = repo
	// Restrict PATH to /usr/bin:/bin so squad is unreachable.
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + os.Getenv("HOME")}
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git commit -m 'feat: x'"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit must pass when squad is not on PATH; got %v: %s", err, out)
	}
}

func TestAgentsMdHook_DashLintClean(t *testing.T) {
	requireDash(t)
	out, err := exec.Command("dash", "-n", fixturePathInRepo(t, "pre_commit_agents_md.sh")).CombinedOutput()
	if err != nil {
		t.Fatalf("dash -n: %v\n%s", err, out)
	}
}
