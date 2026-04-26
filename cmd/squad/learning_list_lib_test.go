package main

import (
	"context"
	"testing"
)

func TestLearningList_PureFiltersByArea(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "gotchas/proposed/a.md", "gotcha", "a", "store", "proposed")
	mkLearning(t, repo, "gotchas/approved/b.md", "gotcha", "b", "store", "approved")
	mkLearning(t, repo, "patterns/approved/c.md", "pattern", "c", "chat", "approved")

	res, err := LearningList(context.Background(), LearningListArgs{
		RepoRoot: repo,
		Area:     "store",
	})
	if err != nil {
		t.Fatalf("LearningList: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("Count=%d want 2", res.Count)
	}
	for _, l := range res.Items {
		if l.Area != "store" {
			t.Errorf("unexpected area %q", l.Area)
		}
	}
}

func TestLearningList_PureFiltersByStateAndKind(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "gotchas/proposed/a.md", "gotcha", "a", "store", "proposed")
	mkLearning(t, repo, "gotchas/approved/b.md", "gotcha", "b", "store", "approved")
	mkLearning(t, repo, "patterns/approved/c.md", "pattern", "c", "chat", "approved")

	res, err := LearningList(context.Background(), LearningListArgs{
		RepoRoot: repo,
		State:    "approved",
		Kind:     "pattern",
	})
	if err != nil {
		t.Fatalf("LearningList: %v", err)
	}
	if res.Count != 1 || res.Items[0].Slug != "c" {
		t.Fatalf("want only pattern/c, got %+v", res.Items)
	}
}
