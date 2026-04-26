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

func TestLearningPropose_PureWritesStub(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	res, err := LearningPropose(context.Background(), LearningProposeArgs{
		RepoRoot:  repo,
		Kind:      "gotcha",
		Slug:      "sqlite-busy-on-fork",
		Title:     "SQLITE_BUSY across fork",
		Area:      "store",
		SessionID: "test-session",
		CreatedBy: "agent-x",
		Now:       func() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("LearningPropose: %v", err)
	}
	if res == nil || res.Learning == nil {
		t.Fatalf("nil result/learning: %+v", res)
	}
	want := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed", "sqlite-busy-on-fork.md")
	if res.Path != want {
		t.Errorf("Path=%q want %q", res.Path, want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read %s: %v", want, err)
	}
	for _, w := range []string{
		"kind: gotcha", "slug: sqlite-busy-on-fork",
		"title: SQLITE_BUSY across fork", "area: store",
		"state: proposed", "## Looks like", "## Is", "## So",
	} {
		if !strings.Contains(string(body), w) {
			t.Errorf("stub missing %q\n---\n%s", w, body)
		}
	}
	if res.Learning.Slug != "sqlite-busy-on-fork" || res.Learning.Area != "store" {
		t.Errorf("Learning fields wrong: %+v", res.Learning)
	}
}

func TestLearningPropose_PureRejectsCollisionAcrossStates(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "patterns/approved/boot-context.md", "pattern", "boot-context", "boot", "approved")

	_, err := LearningPropose(context.Background(), LearningProposeArgs{
		RepoRoot:  repo,
		Kind:      "gotcha",
		Slug:      "boot-context",
		Title:     "x",
		Area:      "boot",
		CreatedBy: "agent-x",
	})
	if !errors.Is(err, ErrSlugCollision) {
		t.Fatalf("err=%v want ErrSlugCollision", err)
	}
	if !strings.Contains(err.Error(), "boot-context") {
		t.Errorf("error should mention slug, got %v", err)
	}
}
