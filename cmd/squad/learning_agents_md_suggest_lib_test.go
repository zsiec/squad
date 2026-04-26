package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentsMdSuggest_PureWritesProposal(t *testing.T) {
	repo := setupSquadRepo(t)
	diffPath := filepath.Join(repo, "patch.diff")
	if err := os.WriteFile(diffPath, []byte(agentsMdDiff), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LearningAgentsMdSuggest(context.Background(), LearningAgentsMdSuggestArgs{
		RepoRoot:  repo,
		DiffPath:  diffPath,
		Rationale: "agents skip §0 fast-tier read; pin a forward reference",
		CreatedBy: "agent-x",
		Now:       func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("LearningAgentsMdSuggest: %v", err)
	}
	if res.ID != "20260425T120000Z-agents-md-edit" {
		t.Errorf("ID=%q", res.ID)
	}
	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read %s: %v", res.Path, err)
	}
	for _, w := range []string{"§0.5 — Read learnings", "agents skip §0 fast-tier", "--- a/AGENTS.md", "+++ b/AGENTS.md"} {
		if !strings.Contains(string(body), w) {
			t.Errorf("missing %q in proposal:\n%s", w, body)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); err == nil {
		// suggest must never write AGENTS.md itself
		got, _ := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
		if strings.Contains(string(got), "§0.5") {
			t.Errorf("AGENTS.md was modified directly; forbidden")
		}
	}
}

func TestAgentsMdSuggest_PureRequiresDiffPath(t *testing.T) {
	repo := setupSquadRepo(t)
	_, err := LearningAgentsMdSuggest(context.Background(), LearningAgentsMdSuggestArgs{
		RepoRoot: repo, Rationale: "x",
	})
	if !errors.Is(err, ErrDiffPathRequired) {
		t.Fatalf("err=%v want ErrDiffPathRequired", err)
	}
}

func TestAgentsMdSuggest_PureRequiresRationale(t *testing.T) {
	repo := setupSquadRepo(t)
	_, err := LearningAgentsMdSuggest(context.Background(), LearningAgentsMdSuggestArgs{
		RepoRoot: repo, DiffPath: "/dev/null",
	})
	if !errors.Is(err, ErrRationaleRequired) {
		t.Fatalf("err=%v want ErrRationaleRequired", err)
	}
}

func TestAgentsMdSuggest_PureMissingDiffFile(t *testing.T) {
	repo := setupSquadRepo(t)
	_, err := LearningAgentsMdSuggest(context.Background(), LearningAgentsMdSuggestArgs{
		RepoRoot: repo, DiffPath: filepath.Join(repo, "nonexistent.diff"), Rationale: "x",
	})
	if !errors.Is(err, ErrDiffFileMissing) {
		t.Fatalf("err=%v want ErrDiffFileMissing", err)
	}
}
