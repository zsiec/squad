package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningApprove_PureMovesAndRegeneratesSkill(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "gotchas/proposed/sqlite-busy.md", "gotcha", "sqlite-busy", "store", "proposed")

	res, err := LearningApprove(context.Background(), LearningApproveArgs{
		RepoRoot: repo, Slug: "sqlite-busy",
	})
	if err != nil {
		t.Fatalf("LearningApprove: %v", err)
	}
	want := filepath.Join(repo, ".squad", "learnings", "gotchas", "approved", "sqlite-busy.md")
	if res.Path != want {
		t.Errorf("Path=%q want %q", res.Path, want)
	}
	old := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed", "sqlite-busy.md")
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("expected old gone: %v", err)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "state: approved") {
		t.Errorf("state line not rewritten:\n%s", body)
	}
	skill := filepath.Join(repo, ".claude", "skills", "squad-learnings.md")
	if _, err := os.Stat(skill); err != nil {
		t.Errorf("expected skill regenerated at %s, got %v", skill, err)
	}
}

func TestLearningApprove_PureRejectsNonProposed(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "patterns/approved/x.md", "pattern", "x", "store", "approved")

	_, err := LearningApprove(context.Background(), LearningApproveArgs{
		RepoRoot: repo, Slug: "x",
	})
	if !errors.Is(err, ErrNotProposed) {
		t.Fatalf("err=%v want ErrNotProposed", err)
	}
}

func TestLearningApprove_PureAmbiguousSlug(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "gotchas/proposed/x.md", "gotcha", "x", "a", "proposed")
	mkLearning(t, repo, "patterns/proposed/x.md", "pattern", "x", "a", "proposed")

	_, err := LearningApprove(context.Background(), LearningApproveArgs{
		RepoRoot: repo, Slug: "x",
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("want ambiguity error, got %v", err)
	}
}
