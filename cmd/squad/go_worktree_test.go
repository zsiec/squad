package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/identity"
)

// TestGoCmd_HonorsDefaultWorktreePerClaim is the regression contract
// for the gap between `squad claim` and `squad go`: setting
// `agent.default_worktree_per_claim: true` in `.squad/config.yaml`
// must produce an isolated worktree on every claim, regardless of
// which verb makes it. Pre-fix, `runGo` called `bc.store.Claim`
// directly with the worktree arg hard-coded to false, so the config
// was silently ignored on the recommended one-command flow.
func TestGoCmd_HonorsDefaultWorktreePerClaim(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-worktree-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))
	writeItemFile(t, repoDir, "FEAT-700-pick-me.md",
		"---\nid: FEAT-700\ntitle: pick me\ntype: feature\npriority: P0\nstatus: open\nestimate: 1h\n---\n\n## Acceptance criteria\n- [ ] do the thing\n")

	t.Chdir(repoDir)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	if err := root.Execute(); err != nil {
		t.Fatalf("squad go: %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "FEAT-700") {
		t.Fatalf("expected FEAT-700 to be claimed; got %s", out.String())
	}

	agentID, err := identity.AgentID()
	if err != nil {
		t.Fatalf("identity.AgentID: %v", err)
	}
	wantPath := filepath.Join(repoDir, ".squad", "worktrees", agentID+"-FEAT-700")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected worktree at %s after `squad go`; stat err=%v\nsquad go output:\n%s",
			wantPath, err, out.String())
	}
}

// TestGoCmd_DoesNotProvisionWorktreeWhenDefaultDisabled pins the
// override path: with the config flipped off, `squad go` claims the
// item without a worktree (back to the pre-flip behavior).
func TestGoCmd_DoesNotProvisionWorktreeWhenDefaultDisabled(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-go-worktree-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)

	first := newInitCmd()
	first.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))

	cfgPath := filepath.Join(repoDir, ".squad", "config.yaml")
	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	// Match the actual key line (leading two-space indent under the
	// `agent:` block), not the documentation comment higher up that
	// also contains the literal `default_worktree_per_claim: true`.
	flipped := strings.Replace(string(body),
		"  default_worktree_per_claim: true",
		"  default_worktree_per_claim: false", 1)
	if flipped == string(body) {
		t.Fatalf("scaffold config did not contain the agent: key line; cannot flip:\n%s", body)
	}
	if err := os.WriteFile(cfgPath, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}

	writeItemFile(t, repoDir, "FEAT-701-pick-me.md",
		"---\nid: FEAT-701\ntitle: pick me\ntype: feature\npriority: P0\nstatus: open\nestimate: 1h\n---\n\n## Acceptance criteria\n- [ ] do the thing\n")

	t.Chdir(repoDir)
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"go"})
	if err := root.Execute(); err != nil {
		t.Fatalf("squad go: %v\nout=%s", err, out.String())
	}
	worktreesDir := filepath.Join(repoDir, ".squad", "worktrees")
	if entries, _ := os.ReadDir(worktreesDir); len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("no worktree expected when default is false; got %v", names)
	}
}
