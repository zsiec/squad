package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningReject_ArchivesNotDeletes(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "patterns/proposed/x.md", "pattern", "x", "store", "proposed")

	root := newRootCmd()
	root.SetArgs([]string{"learning", "reject", "x", "--reason", "duplicates approved/y"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	old := filepath.Join(repo, ".squad", "learnings", "patterns", "proposed", "x.md")
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("expected proposed/x.md gone, got %v", err)
	}
	archived := filepath.Join(repo, ".squad", "learnings", "patterns", "rejected", "x.md")
	body, err := os.ReadFile(archived)
	if err != nil {
		t.Fatalf("expected archived file at %s, got %v", archived, err)
	}
	s := string(body)
	for _, want := range []string{"state: rejected", "## When", "duplicates approved/y"} {
		if !strings.Contains(s, want) {
			t.Errorf("rejected file missing %q:\n%s", want, s)
		}
	}
}

func TestLearningReject_RequiresReason(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "patterns/proposed/x.md", "pattern", "x", "store", "proposed")

	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "reject", "x"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error()+stderr.String(), "reason") {
		t.Fatalf("want reason error, got %v / %s", err, stderr.String())
	}
}

func TestLearningReject_RefusesAlreadyApproved(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "patterns/approved/x.md", "pattern", "x", "store", "approved")

	root := newRootCmd()
	root.SetArgs([]string{"learning", "reject", "x", "--reason", "obsolete"})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected error rejecting an already-approved learning")
	}
}
