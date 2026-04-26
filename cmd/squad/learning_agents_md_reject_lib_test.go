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

func TestAgentsMdReject_PureArchives(t *testing.T) {
	repo := setupSquadRepo(t)
	id := "20260425T120000Z-test"
	dir := filepath.Join(repo, ".squad", "learnings", "agents-md", "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(seedAgentsMdProposal), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LearningAgentsMdReject(context.Background(), LearningAgentsMdRejectArgs{
		RepoRoot: repo, ID: id, Reason: "out of scope",
		Now: func() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("LearningAgentsMdReject: %v", err)
	}
	want := filepath.Join(repo, ".squad", "learnings", "agents-md", "rejected", id+".md")
	if res.Path != want {
		t.Errorf("Path=%q want %q", res.Path, want)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".md")); !os.IsNotExist(err) {
		t.Errorf("expected source removed, got %v", err)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"state: rejected", "out of scope", "## Rejection note (2026-04-25)"} {
		if !strings.Contains(string(body), w) {
			t.Errorf("missing %q in rejected:\n%s", w, body)
		}
	}
}

func TestAgentsMdReject_PureRequiresReason(t *testing.T) {
	repo := setupSquadRepo(t)
	_, err := LearningAgentsMdReject(context.Background(), LearningAgentsMdRejectArgs{
		RepoRoot: repo, ID: "anything",
	})
	if !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("err=%v want ErrReasonRequired", err)
	}
}

func TestAgentsMdReject_PureProposalNotFound(t *testing.T) {
	repo := setupSquadRepo(t)
	_, err := LearningAgentsMdReject(context.Background(), LearningAgentsMdRejectArgs{
		RepoRoot: repo, ID: "nonexistent", Reason: "x",
	})
	if !errors.Is(err, ErrProposalNotFound) {
		t.Fatalf("err=%v want ErrProposalNotFound", err)
	}
}
