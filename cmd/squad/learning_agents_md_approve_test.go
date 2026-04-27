package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

const cleanDiff = `--- a/AGENTS.md
+++ b/AGENTS.md
@@ -1,3 +1,4 @@
 # agents

 body
+extra
`

func TestAgentsMdApprove_BranchesAndArchivesWithoutRemote(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mustGit(t, repo, "checkout", "-b", "main")
	agents := filepath.Join(repo, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# agents\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "AGENTS.md")
	mustGit(t, repo, "commit", "-m", "seed")

	diffPath := filepath.Join(repo, "p.diff")
	if err := os.WriteFile(diffPath, []byte(cleanDiff), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRootCmd()
	root.SetArgs([]string{"learning", "agents-md-suggest", "--diff", diffPath, "--rationale", "test"})
	if err := root.Execute(); err != nil {
		t.Fatalf("suggest: %v", err)
	}

	proposed := filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed")
	entries, _ := os.ReadDir(proposed)
	if len(entries) == 0 {
		t.Fatal("no proposal written")
	}
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	root = newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"learning", "agents-md-approve", id})
	if err := root.Execute(); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// Working tree on the original branch is untouched — the edit landed on
	// the new squad branch, not in-place.
	got, _ := os.ReadFile(agents)
	if strings.Contains(string(got), "extra") {
		t.Errorf("approve should not modify the working tree on the original branch:\n%s", got)
	}
	branchName := "squad/learning-agents-md-" + id
	logOut := mustGitOut(t, repo, "log", branchName, "--oneline", "-1")
	if !strings.Contains(logOut, "agents-md") {
		t.Errorf("expected commit on %s mentioning agents-md, got: %s", branchName, logOut)
	}
	// The branch carries the patched AGENTS.md.
	showOut := mustGitOut(t, repo, "show", branchName+":AGENTS.md")
	if !strings.Contains(showOut, "extra") {
		t.Errorf("squad branch is missing the patched line:\n%s", showOut)
	}
	currentBranch := strings.TrimSpace(mustGitOut(t, repo, "rev-parse", "--abbrev-ref", "HEAD"))
	if currentBranch != "main" {
		t.Errorf("approve did not return to original branch: now on %q", currentBranch)
	}
	out := stdout.String()
	if !strings.Contains(out, "no remote") && !strings.Contains(out, "PR skipped") {
		t.Errorf("expected stdout to call out the no-remote / PR-skipped case, got:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(proposed, id+".md")); !os.IsNotExist(err) {
		t.Errorf("expected proposal moved out of proposed/, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".squad", "learnings", "agents-md", "applied", id+".md")); err != nil {
		t.Errorf("expected applied/%s.md, got %v", id, err)
	}
}

func mustGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func TestAgentsMdApprove_OnConflictLeavesProposal(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	agents := filepath.Join(repo, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# agents\n\nbody changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "AGENTS.md")
	mustGit(t, repo, "commit", "-m", "seed")

	diffPath := filepath.Join(repo, "p.diff")
	if err := os.WriteFile(diffPath, []byte(cleanDiff), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRootCmd()
	root.SetArgs([]string{"learning", "agents-md-suggest", "--diff", diffPath, "--rationale", "x"})
	if err := root.Execute(); err != nil {
		t.Fatalf("suggest: %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	root = newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "agents-md-approve", id})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if _, sErr := os.Stat(filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed", id+".md")); sErr != nil {
		t.Errorf("expected proposal preserved on conflict, got %v", sErr)
	}
	if !strings.Contains(stderr.String()+err.Error(), "apply") {
		t.Errorf("expected git-apply error, got %v / %s", err, stderr.String())
	}
}
