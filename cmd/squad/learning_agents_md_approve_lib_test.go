package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsMdApprove_PureAppliesAndArchives(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	agents := filepath.Join(repo, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# agents\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "AGENTS.md")
	mustGit(t, repo, "commit", "-m", "seed")

	id := "20260425T120000Z-test"
	dir := filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: " + id + "\nkind: agents-md-suggestion\ncreated: 2026-04-25\ncreated_by: agent-x\nstate: proposed\n---\n\n## Rationale\n\nx\n\n## Diff\n\n```diff\n" + cleanDiff + "```\n"
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LearningAgentsMdApprove(context.Background(), LearningAgentsMdApproveArgs{
		RepoRoot: repo, ID: id,
	})
	if err != nil {
		t.Fatalf("LearningAgentsMdApprove: %v", err)
	}
	want := filepath.Join(repo, ".squad", "learnings", "agents-md", "applied", id+".md")
	if res.AppliedPath != want {
		t.Errorf("AppliedPath=%q want %q", res.AppliedPath, want)
	}
	got, _ := os.ReadFile(agents)
	if !strings.Contains(string(got), "extra") {
		t.Errorf("AGENTS.md not patched:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".md")); !os.IsNotExist(err) {
		t.Errorf("expected proposal removed, got %v", err)
	}
	applied, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read applied: %v", err)
	}
	if !strings.Contains(string(applied), "state: applied") {
		t.Errorf("state line not rewritten:\n%s", applied)
	}
}

func TestAgentsMdApprove_PureProposalNotFound(t *testing.T) {
	repo := setupSquadRepo(t)
	_, err := LearningAgentsMdApprove(context.Background(), LearningAgentsMdApproveArgs{
		RepoRoot: repo, ID: "nope",
	})
	if !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("err=%v want ErrProposalNotFound", err)
	}
}

func TestAgentsMdApprove_PureConflictPreservesProposal(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	agents := filepath.Join(repo, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# agents\n\nbody changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", "AGENTS.md")
	mustGit(t, repo, "commit", "-m", "seed")

	id := "20260425T120000Z-conflict"
	dir := filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: " + id + "\nkind: agents-md-suggestion\ncreated: 2026-04-25\ncreated_by: agent-x\nstate: proposed\n---\n\n## Diff\n\n```diff\n" + cleanDiff + "```\n"
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LearningAgentsMdApprove(context.Background(), LearningAgentsMdApproveArgs{
		RepoRoot: repo, ID: id,
	})
	if !errors.Is(err, ErrApplyFailed) {
		t.Fatalf("err=%v want ErrApplyFailed", err)
	}
	if _, sErr := os.Stat(filepath.Join(dir, id+".md")); sErr != nil {
		t.Errorf("expected proposal preserved on conflict, got %v", sErr)
	}
}
