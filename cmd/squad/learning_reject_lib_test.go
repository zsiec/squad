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

func TestLearningReject_PureArchives(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "patterns/proposed/x.md", "pattern", "x", "store", "proposed")

	res, err := LearningReject(context.Background(), LearningRejectArgs{
		RepoRoot: repo, Slug: "x", Reason: "duplicates approved/y",
		Now: func() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("LearningReject: %v", err)
	}
	want := filepath.Join(repo, ".squad", "learnings", "patterns", "rejected", "x.md")
	if res.Path != want {
		t.Errorf("Path=%q want %q", res.Path, want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"state: rejected", "duplicates approved/y", "## Rejection note (2026-04-25)"} {
		if !strings.Contains(string(body), w) {
			t.Errorf("rejected missing %q:\n%s", w, body)
		}
	}
}

func TestLearningReject_PureRequiresReason(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "patterns/proposed/x.md", "pattern", "x", "store", "proposed")

	_, err := LearningReject(context.Background(), LearningRejectArgs{
		RepoRoot: repo, Slug: "x",
	})
	if !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("err=%v want ErrReasonRequired", err)
	}
}

func TestLearningReject_PureRefusesNonProposed(t *testing.T) {
	repo := setupSquadRepo(t)
	mkLearning(t, repo, "patterns/approved/x.md", "pattern", "x", "store", "approved")

	_, err := LearningReject(context.Background(), LearningRejectArgs{
		RepoRoot: repo, Slug: "x", Reason: "obsolete",
	})
	if !errors.Is(err, ErrNotProposed) {
		t.Fatalf("err=%v want ErrNotProposed", err)
	}
}
