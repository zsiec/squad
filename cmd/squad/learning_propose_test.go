package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningPropose_WritesStub(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "propose", "gotcha", "sqlite-busy-on-fork",
		"--title", "SQLITE_BUSY across fork", "--area", "store"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, stderr.String())
	}

	want := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed", "sqlite-busy-on-fork.md")
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
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want path", stdout.String())
	}
}

func TestLearningPropose_RefusesClobber(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	args := []string{"learning", "propose", "pattern", "boot-context",
		"--title", "x", "--area", "boot"}

	root := newRootCmd()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("first propose: %v", err)
	}

	root = newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error()+stderr.String(), "exists") {
		t.Fatalf("want 'exists' error, got %v / %s", err, stderr.String())
	}
}

func TestLearningPropose_RefusesClobberAcrossStates(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	mkLearning(t, repo, "patterns/approved/boot-context.md", "pattern", "boot-context", "boot", "approved")

	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "propose", "gotcha", "boot-context",
		"--title", "x", "--area", "boot"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error()+stderr.String(), "exists") {
		t.Fatalf("want collision error mentioning existing learning, got %v / %s", err, stderr.String())
	}
	if !strings.Contains(err.Error()+stderr.String(), "boot-context") {
		t.Errorf("error should mention the existing slug, got %v / %s", err, stderr.String())
	}
}

func TestLearningPropose_RejectsBadKind(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "propose", "rumor", "x",
		"--title", "t", "--area", "a"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("want kind error")
	}
	if !strings.Contains(err.Error()+stderr.String(), "kind") {
		t.Errorf("want 'kind' error, got %v / %s", err, stderr.String())
	}
}
